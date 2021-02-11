package controllers

import (
	"fmt"
	"strings"

	cpv2 "github.com/eclipse-iofog/iofog-operator/v2/apis/controlplanes/v2"
	"github.com/eclipse-iofog/iofog-operator/v2/controllers/controlplanes/router"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s"

	"github.com/skupperproject/skupper-cli/pkg/certs"
	corev1 "k8s.io/api/core/v1"
)

const (
	loadBalancerTimeout   = 360
	errProxyRouterMissing = "missing Proxy.Router data for non LoadBalancer Router service"
)

func reconcileRoutine(recon func() error, errCh chan error) {
	errCh <- recon()
}

func (r *ControlPlaneReconciler) reconcileIofogController() error {
	// Configure Controller
	ms := newControllerMicroservice(&controllerMicroserviceConfig{
		replicas:         r.cp.Spec.Replicas.Controller,
		image:            r.cp.Spec.Images.Controller,
		imagePullSecret:  r.cp.Spec.Images.PullSecret,
		proxyImage:       r.cp.Spec.Images.Proxy,
		routerImage:      r.cp.Spec.Images.Router,
		db:               &r.cp.Spec.Database,
		serviceType:      r.cp.Spec.Services.Controller.Type,
		loadBalancerAddr: r.cp.Spec.Services.Controller.Address,
		httpPortAddr:     r.cp.Spec.Ingresses.HTTPProxy.Address,
		tcpPortAddr:      r.cp.Spec.Ingresses.TCPProxy.Address,
		tcpAllocatorHost: r.cp.Spec.Ingresses.TCPProxy.TCPAllocatorHost,
		tcpAllocatorPort: r.cp.Spec.Ingresses.TCPProxy.TCPAllocatorPort,
		ecnID:            r.cp.Spec.Ingresses.TCPProxy.EcnID,
		pidBaseDir:       r.cp.Spec.Controller.PidBaseDir,
		ecnViewerPort:    r.cp.Spec.Controller.EcnViewerPort,
		portProvider:     r.cp.Spec.Controller.PortProvider,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(ms); err != nil {
		return err
	}

	// PVC
	if err := r.createPersistentVolumeClaims(ms); err != nil {
		return err
	}

	// Connect to cluster
	k8sClient, err := k8sclient.NewInCluster()
	if err != nil {
		return err
	}

	// Wait for Pods
	if err := k8sClient.WaitForPod(r.cp.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}

	// Wait for external IP of LB Service
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		_, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return err
		}
	}

	// Instantiate Controller client
	ctrlPort, err := getControllerPort(ms)
	if err != nil {
		return err
	}
	host := fmt.Sprintf("%s.%s.svc.cluster.local", ms.name, r.cp.ObjectMeta.Namespace)
	iofogClient := iofogclient.New(iofogclient.Options{Endpoint: fmt.Sprintf("%s:%d", host, ctrlPort)})

	// Wait for Controller REST API
	if err := r.waitForControllerAPI(iofogClient); err != nil {
		return err
	}

	// Set up user
	if err := r.createIofogUser(iofogClient); err != nil {
		return err
	}

	// Get Router or Router Proxy
	var routerProxy cpv2.RouterIngress
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		ipAddress, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			return err
		}
		routerProxy = cpv2.RouterIngress{
			Ingress: cpv2.Ingress{
				Address: ipAddress,
			},
			MessagePort:  router.MessagePort,
			InteriorPort: router.InteriorPort,
			EdgePort:     router.EdgePort,
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		routerProxy = r.cp.Spec.Ingresses.Router
	} else {
		return fmt.Errorf("reconcile Controller failed: %s", errProxyRouterMissing)
	}
	if err := r.createDefaultRouter(iofogClient, routerProxy); err != nil {
		return err
	}

	return nil
}

func (r *ControlPlaneReconciler) reconcilePortManager() error {
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
		return err
	}
	// Role
	if err := r.createRole(ms); err != nil {
		return err
	}
	// RoleBinding
	if err := r.createRoleBinding(ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return err
	}
	return nil
}

func (r *ControlPlaneReconciler) reconcileRouter() error {
	// Configure
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newRouterMicroservice(routerMicroserviceConfig{
		image:           r.cp.Spec.Images.Router,
		serviceType:     r.cp.Spec.Services.Router.Type,
		volumeMountPath: volumeMountPath,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return err
	}

	// Role
	if err := r.createRole(ms); err != nil {
		return err
	}

	// Role binding
	if err := r.createRoleBinding(ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(ms); err != nil {
		return err
	}

	// Wait for IP
	k8sClient, err := k8sclient.NewInCluster()
	if err != nil {
		return err
	}

	// Wait for external IP of LB Service
	address := ""
	if strings.EqualFold(r.cp.Spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		address, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return err
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		address = r.cp.Spec.Ingresses.Router.Address
	} else {
		return fmt.Errorf("reconcile Router failed: %s", errProxyRouterMissing)
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
		return err
	}

	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return err
	}

	return nil
}
