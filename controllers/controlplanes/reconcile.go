package controllers

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s"
	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
	"github.com/skupperproject/skupper-cli/pkg/certs"
	corev1 "k8s.io/api/core/v1"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
)

const (
	loadBalancerTimeout   = 360
	errProxyRouterMissing = "missing Proxy.Router data for non LoadBalancer Router service"
	errParseControllerURL = "failed to parse Controller endpoint as URL: %s"
)

func reconcileRoutine(recon func() op.Reconciliation, reconChan chan op.Reconciliation) {
	reconChan <- recon()
}

func (r *ControlPlaneReconciler) reconcileIofogController() op.Reconciliation {
	// Configure Controller
	ms := newControllerMicroservice(&controllerMicroserviceConfig{
		replicas:          r.cp.Spec.Replicas.Controller,
		image:             r.cp.Spec.Images.Controller,
		imagePullSecret:   r.cp.Spec.Images.PullSecret,
		proxyImage:        r.cp.Spec.Images.Proxy,
		routerImage:       r.cp.Spec.Images.Router,
		db:                &r.cp.Spec.Database,
		serviceType:       r.cp.Spec.Services.Controller.Type,
		loadBalancerAddr:  r.cp.Spec.Services.Controller.Address,
		portAllocatorHost: r.cp.Spec.Controller.PortAllocatorHost,
		ecn:               r.cp.Spec.Controller.ECNName,
		pidBaseDir:        r.cp.Spec.Controller.PidBaseDir,
		ecnViewerPort:     r.cp.Spec.Controller.EcnViewerPort,
		portProvider:      r.cp.Spec.Controller.PortProvider,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// PVC
	if err := r.createPersistentVolumeClaims(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	alreadyExists, err := r.deploymentExists(r.cp.Namespace, ms.name)
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// The deployment was just created, requeue to hide latency
	if !alreadyExists {
		return op.ReconcileWithRequeue(time.Second * 5)
	}
	// Instantiate Controller client
	ctrlPort, err := getControllerPort(ms)
	if err != nil {
		return op.ReconcileWithError(err)
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", ms.name, r.cp.ObjectMeta.Namespace)
	baseURL := fmt.Sprintf("http://%s:%d/api/v3", host, ctrlPort)
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return op.ReconcileWithError(fmt.Errorf(errParseControllerURL, baseURL, err.Error()))
	}
	iofogClient := iofogclient.New(iofogclient.Options{
		Timeout: 1,
		BaseURL: *parsedURL,
	})

	// Wait for Controller REST API
	if _, err = iofogClient.GetStatus(); err != nil {
		r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s: %s", r.cp.Name, err.Error()))
		return op.ReconcileWithRequeue(time.Second * 3)
	}

	// Set up user
	if err := r.createIofogUser(iofogClient); err != nil {
		r.log.Info(fmt.Sprintf("Could not create user for ControlPlane %s: %s", r.cp.Name, err.Error()))
		return op.ReconcileWithRequeue(time.Second * 3)
	}

	// Connect to cluster
	k8sClient, err := newK8sClient()
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Get Router or Router Proxy
	var routerProxy cpv3.RouterIngress
	if strings.EqualFold(r.cp.Spec.Services.Router.Type, string(corev1.ServiceTypeLoadBalancer)) {
		routerAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
		routerProxy = cpv3.RouterIngress{
			Ingress: cpv3.Ingress{
				Address: routerAddr,
			},
			MessagePort:  router.MessagePort,
			InteriorPort: router.InteriorPort,
			EdgePort:     router.EdgePort,
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		routerProxy = r.cp.Spec.Ingresses.Router
	} else {
		err := fmt.Errorf("reconcile Controller failed: %s", errProxyRouterMissing)
		return op.ReconcileWithError(err)
	}
	if err := r.createDefaultRouter(iofogClient, routerProxy); err != nil {
		return op.ReconcileWithError(err)
	}

	// Wait for Controller LB to actually work
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		controllerAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, controllerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
		baseURL := fmt.Sprintf("https://%s:%d/api/v3", controllerAddr, ctrlPort)
		parsedURL, err := url.Parse(baseURL)
		if err != nil {
			return op.ReconcileWithError(fmt.Errorf(errParseControllerURL, baseURL, err.Error()))
		}
		iofogClient = iofogclient.New(iofogclient.Options{
			BaseURL: *parsedURL,
			Timeout: 1,
		})
		if _, err = iofogClient.GetStatus(); err != nil {
			r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s via LoadBalancer: %s", r.cp.Name, err.Error()))
			return op.ReconcileWithRequeue(time.Second * 3)
		}
	}

	return op.Continue()
}

func (r *ControlPlaneReconciler) reconcilePortManager() op.Reconciliation {
	ms := newPortManagerMicroservice(&portManagerConfig{
		image:            r.cp.Spec.Images.PortManager,
		proxyImage:       r.cp.Spec.Images.Proxy,
		httpProxyAddress: r.cp.Spec.Ingresses.HTTPProxy.Address,
		tcpProxyAddress:  r.cp.Spec.Ingresses.TCPProxy.Address,
		watchNamespace:   r.cp.ObjectMeta.Namespace,
		userEmail:        r.cp.Spec.User.Email,
		userPass:         r.cp.Spec.User.Password,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	// Role
	if err := r.createRole(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	// RoleBinding
	if err := r.createRoleBinding(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return op.ReconcileWithError(err)
	}
	return op.Continue()
}

func (r *ControlPlaneReconciler) reconcileRouter() op.Reconciliation {
	// Configure
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newRouterMicroservice(routerMicroserviceConfig{
		image:           r.cp.Spec.Images.Router,
		serviceType:     r.cp.Spec.Services.Router.Type,
		volumeMountPath: volumeMountPath,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role
	if err := r.createRole(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Role binding
	if err := r.createRoleBinding(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Service
	if err := r.createService(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Wait for IP
	k8sClient, err := newK8sClient()
	if err != nil {
		return op.ReconcileWithError(err)
	}

	// Wait for external IP of LB Service
	address := ""
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		address, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		address = r.cp.Spec.Ingresses.Router.Address
	} else {
		err = fmt.Errorf("reconcile Router failed: %s", errProxyRouterMissing)
		return op.ReconcileWithError(err)
	}

	// Secrets
	// CA
	caName := "router-ca"
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		secret := certs.GenerateSecret("router-"+suffix, address, address, &caSecret)
		secret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
		ms.secrets = append(ms.secrets, secret)
	}

	// Create secrets
	if err := r.createSecrets(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return op.ReconcileWithError(err)
	}

	return op.Continue()
}

func newK8sClient() (*k8sclient.Client, error) {
	kubeConf := os.Getenv("KUBECONFIG")
	if kubeConf == "" {
		return k8sclient.NewInCluster()
	}
	return k8sclient.New(kubeConf)
}
