package controlplane

import (
	"context"
	"fmt"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/client"
	k8sclient "github.com/eclipse-iofog/iofog-go-sdk/v2/pkg/k8s"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/apis/iofog"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/skupperproject/skupper-cli/pkg/certs"
)

func (r *ReconcileControlPlane) reconcileIofogController(cp *iofog.ControlPlane) error {
	// Configure
	ms := newControllerMicroservice(controllerMicroserviceConfig{
		replicas:        cp.Spec.Replicas.Controller,
		image:           cp.Spec.Images.Controller,
		imagePullSecret: cp.Spec.Images.PullSecret,
		db:              &cp.Spec.Database,
		serviceType:     cp.Spec.Services.Controller.Type,
		loadBalancerIP:  cp.Spec.Services.Controller.IP,
	})
	r.apiEndpoint = fmt.Sprintf("%s:%d", ms.name, ms.ports[0])
	r.iofogClient = iofogclient.New(iofogclient.Options{Endpoint: r.apiEndpoint})

	// Service Account
	if err := r.createServiceAccount(cp, ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(cp, ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(cp, ms); err != nil {
		return err
	}

	// PVC
	if err := r.createPersistentVolumeClaims(cp, ms); err != nil {
		return err
	}

	// Connect to cluster
	k8sClient, err := k8sclient.NewInCluster()
	if err != nil {
		return err
	}

	// Wait for Pods
	if err = k8sClient.WaitForPod(cp.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}

	// Wait for external IP of LB Service
	if cp.Spec.Services.Controller.Type == string(corev1.ServiceTypeLoadBalancer) {
		_, err = k8sClient.WaitForLoadBalancer(cp.ObjectMeta.Namespace, ms.name, 240)
		if err != nil {
			return err
		}
	}

	// Wait for Controller REST API
	if err = r.waitForControllerAPI(); err != nil {
		return err
	}

	// Set up user
	if err = r.createIofogUser(&cp.Spec.User); err != nil {
		return err
	}

	// Get Router IP
	routerIP, err := k8sClient.WaitForLoadBalancer(cp.Namespace, routerName, 120)
	if err != nil {
		return err
	}
	// Create default router
	if err = r.createDefaultRouter(&cp.Spec.User, routerIP); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) reconcilePortManager(cp *iofog.ControlPlane) error {
	ms := newPortManagerMicroservice(portManagerConfig{
		image:          cp.Spec.Images.PortManager,
		proxyImage:     cp.Spec.Images.Proxy,
		watchNamespace: cp.ObjectMeta.Namespace,
		userEmail:      cp.Spec.User.Email,
		userPass:       cp.Spec.User.Password,
	})

	// Service Account
	if err := r.createServiceAccount(cp, ms); err != nil {
		return err
	}
	// TODO: Use Role Binding instead
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(cp, ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(cp, ms); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileControlPlane) reconcileIofogKubelet(cp *iofog.ControlPlane) error {
	// Generate new token if required
	token := ""
	kubeletKey := client.ObjectKey{
		Name:      "kubelet",
		Namespace: cp.ObjectMeta.Namespace,
	}
	dep := appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), kubeletKey, &dep); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// Not found, generate new token
		token = r.iofogClient.GetAccessToken()
	} else {
		// Found, use existing token
		token, err = getKubeletToken(dep.Spec.Template.Spec.Containers)
		if err != nil {
			return err
		}
	}

	// Configure
	ms := newKubeletMicroservice(cp.Spec.Images.Kubelet, cp.ObjectMeta.Namespace, token, r.apiEndpoint)

	// Service Account
	if err := r.createServiceAccount(cp, ms); err != nil {
		return err
	}
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(cp, ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(cp, ms); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileControlPlane) reconcileRouter(cp *iofog.ControlPlane) error {
	// Configure
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newRouterMicroservice(routerMicroserviceConfig{
		image:           cp.Spec.Images.Router,
		serviceType:     cp.Spec.Services.Router.Type,
		ip:              cp.Spec.Services.Router.IP,
		volumeMountPath: volumeMountPath,
	})

	// Service Account
	if err := r.createServiceAccount(cp, ms); err != nil {
		return err
	}

	// Role
	if err := r.createRole(cp, ms); err != nil {
		return err
	}

	// Role binding
	if err := r.createRoleBinding(cp, ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(cp, ms); err != nil {
		return err
	}

	// Wait for IP
	k8sClient, err := k8sclient.NewInCluster()
	if err != nil {
		return err
	}

	ip, err := k8sClient.WaitForLoadBalancer(cp.ObjectMeta.Namespace, ms.name, 120)
	if err != nil {
		return err
	}

	// Secrets
	// CA
	caName := "router-ca"
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = cp.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		secret := certs.GenerateSecret("router-"+suffix, ip, ip, &caSecret)
		secret.ObjectMeta.Namespace = cp.ObjectMeta.Namespace
		ms.secrets = append(ms.secrets, secret)
	}

	// Create secrets
	if err := r.createSecrets(cp, ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(cp, ms); err != nil {
		return err
	}

	return nil
}
