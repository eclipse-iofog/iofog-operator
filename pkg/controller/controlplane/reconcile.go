package controlplane

import (
	"context"
	"fmt"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/skupperproject/skupper-cli/pkg/certs"
)

func reconcileRoutine(recon func() error, errCh chan error) {
	errCh <- recon()
}

func (r *ReconcileControlPlane) reconcileIofogController() error {
	// Configure
	ms := newControllerMicroservice(controllerMicroserviceConfig{
		replicas:         r.cp.Spec.Replicas.Controller,
		image:            r.cp.Spec.Images.Controller,
		imagePullSecret:  r.cp.Spec.Images.PullSecret,
		proxyImage:       r.cp.Spec.Images.Proxy,
		routerImage:      r.cp.Spec.Images.Router,
		db:               &r.cp.Spec.Database,
		serviceType:      r.cp.Spec.Services.Controller.Type,
		loadBalancerAddr: r.cp.Spec.Services.Controller.Address,
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
		_, err = k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, 240)
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

	// Get Router IP
	routerIP, err := k8sClient.WaitForLoadBalancer(r.cp.Namespace, routerName, 240)
	if err != nil {
		return err
	}
	// Create default router
	if err = r.createDefaultRouter(iofogClient, routerIP); err != nil {
		return err
	}

	return r.reconcileIofogKubelet(iofogClient)
}

func (r *ReconcileControlPlane) reconcilePortManager() error {
	ms := newPortManagerMicroservice(portManagerConfig{
		image:            r.cp.Spec.Images.PortManager,
		proxyImage:       r.cp.Spec.Images.Proxy,
		proxyAddress:     r.cp.Spec.Services.Proxy.Address,
		proxyServiceType: r.cp.Spec.Services.Proxy.Type,
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
		if !errors.IsNotFound(err) {
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

	ip, err := k8sClient.WaitForLoadBalancer(r.cp.ObjectMeta.Namespace, ms.name, 120)
	if err != nil {
		return err
	}

	// Secrets
	// CA
	caName := "router-ca"
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = r.cp.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		secret := certs.GenerateSecret("router-"+suffix, ip, ip, &caSecret)
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
