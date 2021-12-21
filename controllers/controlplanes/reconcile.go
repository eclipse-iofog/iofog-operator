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
	errParseControllerURL = "failed to parse Controller endpoint as URL (%s): %s"
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
	iofogClient, fin := r.getIofogClient(host, ctrlPort)
	if fin.IsFinal() {
		return fin
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
			Address:      routerAddr,
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

	r.log.Info(fmt.Sprintf("Waiting for IP/LB Service in iofog-controller reconcile for ControlPlane %s", r.cp.Name))
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		host, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, controllerName, loadBalancerTimeout)
		if err != nil {
			return op.ReconcileWithError(err)
		}
		// Check LB connection works
		if _, fin := r.getIofogClient(host, ctrlPort); fin.IsFinal() {
			r.log.Info(fmt.Sprintf("LB Connection works for ControlPlane %s", r.cp.Name))
			return fin
		}
	}

	r.log.Info(fmt.Sprintf("op.Continue in iofog-controller reconcile for ControlPlane %s", r.cp.Name))
	return op.Continue()
}

func (r *ControlPlaneReconciler) getIofogClient(host string, port int) (*iofogclient.Client, op.Reconciliation) {
	baseURL := fmt.Sprintf("http://%s:%d/api/v3", host, port)
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, op.ReconcileWithError(fmt.Errorf(errParseControllerURL, baseURL, err.Error()))
	}
	iofogClient := iofogclient.New(iofogclient.Options{
		BaseURL: parsedURL,
		Timeout: 1,
	})
	if _, err = iofogClient.GetStatus(); err != nil {
		r.log.Info(fmt.Sprintf("Could not get Controller status for ControlPlane %s: %s", r.cp.Name, err.Error()))
		return nil, op.ReconcileWithRequeue(time.Second * 3)
	}
	return iofogClient, op.Continue()
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

	r.log.Info(fmt.Sprintf("Waiting for IP/LB Service in router reconcile for ControlPlane %s", r.cp.Name))
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
	r.log.Info(fmt.Sprintf("Found address %s for router reconcile for Controlplane %s", address, r.cp.Name))

	// Secrets
	if err = r.createRouterSecrets(ms, address); err != nil {
		return op.ReconcileWithError(err)
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

func (r *ControlPlaneReconciler) createRouterSecrets(ms *microservice, address string) (err error) {
	r.log.Info(fmt.Sprintf("Creating secrets for router reconcile for Controlplane %s", r.cp.Name))
	defer func() {
		if recoverResult := recover(); recoverResult != nil {
			r.log.Info(fmt.Sprintf("Recover result %v for creating secrets for router reconcile for Controlplane %s", recoverResult, r.cp.Name))
			err = fmt.Errorf("createRouterSecrets failed: %v", recoverResult)
		}
	}()
	// CA
	caName := "router-ca"
	r.log.Info(fmt.Sprintf("Generating CA Secret secrets for router reconcile for Controlplane %s", r.cp.Name))
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		r.log.Info(fmt.Sprintf("Generating %s Secret secrets for router reconcile for Controlplane %s", suffix, r.cp.Name))
		secret := certs.GenerateSecret("router-"+suffix, address, address, &caSecret)
		secret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
		ms.secrets = append(ms.secrets, secret)
	}

	r.log.Info(fmt.Sprintf("Secrets generated %v for Controlplane %s", ms.secrets, r.cp.Name))
	return err
}

func newK8sClient() (*k8sclient.Client, error) {
	kubeConf := os.Getenv("KUBECONFIG")
	if kubeConf == "" {
		return k8sclient.NewInCluster()
	}
	return k8sclient.New(kubeConf)
}
