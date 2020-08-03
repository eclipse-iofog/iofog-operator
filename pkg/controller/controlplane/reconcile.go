package controlplane

import (
	"context"
	"errors"
	"fmt"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/apis/iofog"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/controlplane/router"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/skupperproject/skupper-cli/pkg/certs"
)

const (
	loadBalancerTimeout = 360
)

func reconcileRoutine(recon func() error, errCh chan error) {
	errCh <- recon()
}

func (r *ReconcileControlPlane) reconcileIofogController() error {
	// Configure Controller
	ms := newControllerMicroservice(controllerMicroserviceConfig{
		replicas:         r.cp.Spec.Replicas.Controller,
		image:            r.cp.Spec.Images.Controller,
		imagePullSecret:  r.cp.Spec.Images.PullSecret,
		proxyImage:       r.cp.Spec.Images.Proxy,
		routerImage:      r.cp.Spec.Images.Router,
		db:               &r.cp.Spec.Database,
		serviceType:      r.cp.Spec.Services.Controller.Type,
		loadBalancerAddr: r.cp.Spec.Services.Controller.Address,
		httpPortAddr:     r.cp.Spec.Ingresses.HttpProxy.Address,
		tcpPortAddr:      r.cp.Spec.Ingresses.TcpProxy.Address,
		tcpAllocatorHost: r.cp.Spec.Ingresses.TcpProxy.TcpAllocatorHost,
		tcpAllocatorPort: r.cp.Spec.Ingresses.TcpProxy.TcpAllocatorPort,
		ecnId:            r.cp.Spec.Ingresses.TcpProxy.EcnId,
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
	if err = k8sClient.WaitForPod(r.cp.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}

	// Wait for external IP of LB Service
	if r.cp.Spec.Services.Controller.Type == string(corev1.ServiceTypeLoadBalancer) {
		_, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return err
		}
	}

	// Instantiate Controller client
	iofogClient := *iofogclient.New(iofogclient.Options{Endpoint: fmt.Sprintf("%s:%d", ms.name, ms.ports[0])})

	// Wait for Controller REST API
	if err = r.waitForControllerAPI(iofogClient); err != nil {
		return err
	}

	// Set up user
	if iofogClient, err = r.createIofogUser(iofogClient); err != nil {
		return err
	}

	// Get Router or Router Proxy
	var routerProxy iofog.RouterIngress
	if r.cp.Spec.Services.Controller.Type == string(corev1.ServiceTypeLoadBalancer) {
		ipAddress, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, loadBalancerTimeout)
		if err != nil {
			return err
		}
		routerProxy = iofog.RouterIngress{
			Ingress: iofog.Ingress{
				Address: ipAddress,
			},
			HttpPort:     router.HTTPPort,
			MessagePort:  router.MessagePort,
			InteriorPort: router.InteriorPort,
			EdgePort:     router.EdgePort,
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		routerProxy = r.cp.Spec.Ingresses.Router
	} else {
		return errors.New(fmt.Sprintf("Reconcile Controller failed: Missing Proxy.Router data for non LoadBalancer Router service"))
	}
	if err = r.createDefaultRouter(iofogClient, routerProxy); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) reconcilePortManager() error {
	ms := newPortManagerMicroservice(portManagerConfig{
		image:            r.cp.Spec.Images.PortManager,
		proxyImage:       r.cp.Spec.Images.Proxy,
		httpProxyAddress: r.cp.Spec.Ingresses.HttpProxy.Address,
		tcpProxyAddress:  r.cp.Spec.Ingresses.TcpProxy.Address,
		watchNamespace:   r.cp.ObjectMeta.Namespace,
		userEmail:        r.cp.Spec.User.Email,
		userPass:         r.cp.Spec.User.Password,
	})

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return err
	}
	// TODO: Use Role Binding instead
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileControlPlane) reconcileIofogKubelet(iofogClient iofogclient.Client) error {
	// Generate new token if required
	token := ""
	kubeletKey := client.ObjectKey{
		Name:      "kubelet",
		Namespace: r.cp.ObjectMeta.Namespace,
	}
	dep := appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), kubeletKey, &dep); err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		// Not found, generate new token
		token = iofogClient.GetAccessToken()
	} else {
		// Found, use existing token
		token, err = getKubeletToken(dep.Spec.Template.Spec.Containers)
		if err != nil {
			return err
		}
	}

	// Configure
	ms := newKubeletMicroservice(r.cp.Spec.Images.Kubelet, r.cp.ObjectMeta.Namespace, token, iofogClient.GetEndpoint())

	// Service Account
	if err := r.createServiceAccount(ms); err != nil {
		return err
	}
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(ms); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) reconcileRouter() error {
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
	if r.cp.Spec.Services.Controller.Type == string(corev1.ServiceTypeLoadBalancer) {
		address, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, loadBalancerTimeout)
		if err != nil {
			return err
		}
	} else if r.cp.Spec.Ingresses.Router.Address != "" {
		address = r.cp.Spec.Ingresses.Router.Address
	} else {
		return errors.New(fmt.Sprintf("Reconcile Router failed: Missing Proxy.Router data for non LoadBalancer Router service"))
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
