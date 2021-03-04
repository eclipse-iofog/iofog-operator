package controllers

import (
	"fmt"
	"os"
	"strings"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s"
	"github.com/skupperproject/skupper-cli/pkg/certs"
	corev1 "k8s.io/api/core/v1"

	cpv2 "github.com/eclipse-iofog/iofog-operator/v2/apis/controlplanes/v2"
	ctrls "github.com/eclipse-iofog/iofog-operator/v2/controllers"
	"github.com/eclipse-iofog/iofog-operator/v2/controllers/controlplanes/router"
)

const (
	loadBalancerTimeout   = 360
	errProxyRouterMissing = "missing Proxy.Router data for non LoadBalancer Router service"
)

func reconcileRoutine(recon func() ctrls.Reconciliation, reconChan chan ctrls.Reconciliation) {
	reconChan <- recon()
}

func (r *ControlPlaneReconciler) reconcileIofogController() (recon ctrls.Reconciliation) {
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
		recon.Err = err
		return
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		recon.Err = err
		return
	}

	// Service
	if err := r.createService(ms); err != nil {
		recon.Err = err
		return
	}

	// PVC
	if err := r.createPersistentVolumeClaims(ms); err != nil {
		recon.Err = err
		return
	}

	// Connect to cluster
	k8sClient, err := newK8sClient()
	if err != nil {
		recon.Err = err
		return
	}

	// Instantiate Controller client
	ctrlPort, err := getControllerPort(ms)
	if err != nil {
		recon.Err = err
		return
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", ms.name, r.cp.ObjectMeta.Namespace)
	iofogClient := iofogclient.New(iofogclient.Options{Endpoint: fmt.Sprintf("%s:%d", host, ctrlPort)})

	// Wait for Controller REST API
	if _, err = iofogClient.GetStatus(); err != nil {
		r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s: %s", r.cp.Name, err.Error()))
		recon.Result.Requeue = true
		recon.Result.RequeueAfter = time.Second * 5
		return
	}

	// Set up user
	if err := r.createIofogUser(iofogClient); err != nil {
		r.log.Info(fmt.Sprintf("Could not create user for ControlPlane %s: %s", r.cp.Name, err.Error()))
		recon.Result.Requeue = true
		recon.Result.RequeueAfter = time.Second * 5
		return
	}

	// Get Router or Router Proxy
	var routerProxy cpv2.RouterIngress
	if strings.EqualFold(r.cp.Spec.Services.Router.Type, string(corev1.ServiceTypeLoadBalancer)) {
		routerAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			recon.Err = err
			return
		}
		routerProxy = cpv2.RouterIngress{
			Ingress: cpv2.Ingress{
				Address: routerAddr,
			},
			MessagePort:  router.MessagePort,
			InteriorPort: router.InteriorPort,
			EdgePort:     router.EdgePort,
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		routerProxy = r.cp.Spec.Ingresses.Router
	} else {
		recon.Err = fmt.Errorf("reconcile Controller failed: %s", errProxyRouterMissing)
		return
	}
	if err := r.createDefaultRouter(iofogClient, routerProxy); err != nil {
		recon.Err = err
		return
	}

	// Wait for Controller LB to actually work
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		controllerAddr, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			recon.Err = err
			return
		}
		iofogClient = iofogclient.New(iofogclient.Options{Endpoint: fmt.Sprintf("%s:%d", controllerAddr, ctrlPort)})
		if _, err = iofogClient.GetStatus(); err != nil {
			r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s via LoadBalancer: %s", r.cp.Name, err.Error()))
			recon.Result.Requeue = true
			recon.Result.RequeueAfter = time.Second * 5
			return
		}
	}

	return recon
}

func (r *ControlPlaneReconciler) reconcilePortManager() (recon ctrls.Reconciliation) {
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
		recon.Err = err
		return
	}
	// Role
	if err := r.createRole(ms); err != nil {
		recon.Err = err
		return
	}
	// RoleBinding
	if err := r.createRoleBinding(ms); err != nil {
		recon.Err = err
		return
	}
	// Deployment
	if err := r.createDeployment(ms); err != nil {
		recon.Err = err
		return
	}
	return recon
}

func (r *ControlPlaneReconciler) reconcileRouter() (recon ctrls.Reconciliation) {
	// Configure
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newRouterMicroservice(routerMicroserviceConfig{
		image:           r.cp.Spec.Images.Router,
		serviceType:     r.cp.Spec.Services.Router.Type,
		volumeMountPath: volumeMountPath,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		recon.Err = err
		return
	}

	// Role
	if err := r.createRole(ms); err != nil {
		recon.Err = err
		return
	}

	// Role binding
	if err := r.createRoleBinding(ms); err != nil {
		recon.Err = err
		return
	}

	// Service
	if err := r.createService(ms); err != nil {
		recon.Err = err
		return
	}

	// Wait for IP
	k8sClient, err := newK8sClient()
	if err != nil {
		recon.Err = err
		return
	}

	// Wait for external IP of LB Service
	address := ""
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		address, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			recon.Err = err
			return
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		address = r.cp.Spec.Ingresses.Router.Address
	} else {
		recon.Err = fmt.Errorf("reconcile Router failed: %s", errProxyRouterMissing)
		return
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
		recon.Err = err
		return
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		recon.Err = err
		return
	}

	return recon
}

func newK8sClient() (*k8sclient.Client, error) {
	kubeConf := os.Getenv("KUBECONFIG")
	if kubeConf == "" {
		return k8sclient.NewInCluster()
	}
	return k8sclient.New(kubeConf)
}
