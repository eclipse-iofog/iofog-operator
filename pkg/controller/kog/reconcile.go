package kog

import (
	"context"
	"fmt"

	iofogv1 "github.com/eclipse-iofog/iofog-operator/pkg/apis/iofog/v1"
	k8sclient "github.com/eclipse-iofog/iofog-operator/pkg/controller/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/skupperproject/skupper-cli/pkg/certs"
)

func (r *ReconcileKog) reconcileIofogController(kog *iofogv1.Kog) error {
	cp := &kog.Spec.ControlPlane

	// Connect to cluster
	k8sClient, err := k8sclient.New()
	if err != nil {
		return err
	}

	// Get Router IP
	routerIP, err := k8sClient.WaitForLoadBalancer(kog.Namespace, newSkupperMicroservice("", "").name, 120)
	if err != nil {
		return err
	}

	// Configure
	ms := newControllerMicroservice(
		cp.ControllerReplicaCount,
		cp.ControllerImage,
		cp.ImagePullSecret,
		&cp.Database,
		cp.ServiceType,
		cp.LoadBalancerIP,
		routerIP,
	)
	r.apiEndpoint = fmt.Sprintf("%s:%d", ms.name, ms.ports[0])

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(kog, ms); err != nil {
		return err
	}

	// Wait for Pods
	if err = k8sClient.WaitForPod(kog.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}

	// Wait for external IP of LB Service
	if cp.ServiceType == string(corev1.ServiceTypeLoadBalancer) {
		_, err = k8sClient.WaitForLoadBalancer(kog.ObjectMeta.Namespace, ms.name, 240)
		if err != nil {
			return err
		}
	}

	// Wait for Controller REST API
	if err = r.waitForControllerAPI(); err != nil {
		return err
	}

	// Set up user
	if err = r.createIofogUser(&cp.IofogUser); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) reconcilePortManager(kog *iofogv1.Kog) error {
	// TODO: remove hard coded image
	ms := newPortManagerMicroservice(
		kog.Spec.ControlPlane.PortManagerImage,
		kog.ObjectMeta.Namespace,
		kog.Spec.ControlPlane.IofogUser.Email,
		kog.Spec.ControlPlane.IofogUser.Password)

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}
	// TODO: Use Role Binding instead
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(kog, ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}
	return nil
}

func (r *ReconcileKog) reconcileIofogKubelet(kog *iofogv1.Kog) error {
	// Generate new token if required
	token := ""
	kubeletKey := client.ObjectKey{
		Name:      "kubelet",
		Namespace: kog.ObjectMeta.Namespace,
	}
	dep := appsv1.Deployment{}
	if err := r.client.Get(context.TODO(), kubeletKey, &dep); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
		// Not found, generate new token
		token, err = r.getKubeletToken(&kog.Spec.ControlPlane.IofogUser)
		if err != nil {
			return err
		}
	} else {
		// Found, use existing token
		token, err = getKubeletToken(dep.Spec.Template.Spec.Containers)
		if err != nil {
			return err
		}
	}

	// Configure
	ms := newKubeletMicroservice(kog.Spec.ControlPlane.KubeletImage, kog.ObjectMeta.Namespace, token, r.apiEndpoint)

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}
	// ClusterRoleBinding
	if err := r.createClusterRoleBinding(kog, ms); err != nil {
		return err
	}
	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) reconcileSkupper(kog *iofogv1.Kog) error {
	// Configure
	qpidRouterImageName := "quay.io/interconnectedcloud/qdrouterd"
	volumeMountPath := "/etc/qpid-dispatch-certs/"
	ms := newSkupperMicroservice(qpidRouterImageName, volumeMountPath)

	// Service Account
	if err := r.createServiceAccount(kog, ms); err != nil {
		return err
	}

	// Role
	if err := r.createRole(kog, ms); err != nil {
		return err
	}

	// Role binding
	if err := r.createRoleBinding(kog, ms); err != nil {
		return err
	}

	// Service
	if err := r.createService(kog, ms); err != nil {
		return err
	}

	// Wait for IP
	k8sClient, err := k8sclient.New()
	if err != nil {
		return err
	}

	ip, err := k8sClient.WaitForLoadBalancer(kog.ObjectMeta.Namespace, ms.name, 120)
	if err != nil {
		return err
	}

	// Secrets
	// CA
	caName := "skupper-ca"
	caSecret := certs.GenerateCASecret(caName, caName)
	caSecret.ObjectMeta.Namespace = kog.ObjectMeta.Namespace
	ms.secrets = append(ms.secrets, caSecret)

	// AMQPS and Internal
	for _, suffix := range []string{"amqps", "internal"} {
		secret := certs.GenerateSecret("skupper-"+suffix, ip, ip, &caSecret)
		secret.ObjectMeta.Namespace = kog.ObjectMeta.Namespace
		ms.secrets = append(ms.secrets, secret)
	}

	// Create secrets
	if err := r.createSecrets(kog, ms); err != nil {
		return err
	}

	// Deployment
	if err := r.createDeployment(kog, ms); err != nil {
		return err
	}

	return nil
}
