package kog

import (
	"context"

	k8sv1alpha2 "github.com/eclipse-iofog/iofog-operator/pkg/apis/k8s/v1alpha2"
	k8sclient "github.com/eclipse-iofog/iofog-operator/pkg/controller/client"

	iofogclient "github.com/eclipse-iofog/iofogctl/pkg/iofog/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *ReconcileKog) deleteConnector(kog *k8sv1alpha2.Kog, name string) error {
	meta := metav1.ObjectMeta{
		Name:      name,
		Namespace: kog.ObjectMeta.Namespace,
	}
	// Delete deployment
	dep := &appsv1.Deployment{ObjectMeta: meta}
	if err := r.client.Delete(context.Background(), dep); err != nil {
		return err
	}

	// Delete service
	svc := &corev1.Service{ObjectMeta: meta}
	if err := r.client.Delete(context.Background(), svc); err != nil {
		return err
	}

	// Delete service account
	svcAcc := &corev1.ServiceAccount{ObjectMeta: meta}
	if err := r.client.Delete(context.Background(), svcAcc); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) createConnector(kog *k8sv1alpha2.Kog, name string) error {
	// Connect to cluster
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		return err
	}
	ms := newConnectorMicroservice(kog.Spec.Connectors.Image)
	ms.name = name
	// Create
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
	if err := k8sClient.WaitForPod(kog.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}
	// Wait for Service
	ip, err := k8sClient.WaitForService(kog.ObjectMeta.Namespace, ms.name, 240)
	if err != nil {
		return err
	}
	// Log into Controller
	iofogClient := iofogclient.New(r.apiEndpoint)
	if err = iofogClient.Login(iofogclient.LoginRequest{
		Email:    kog.Spec.ControlPlane.IofogUser.Email,
		Password: kog.Spec.ControlPlane.IofogUser.Password,
	}); err != nil {
		return err
	}
	// Provision the Connector
	if err = iofogClient.AddConnector(iofogclient.ConnectorInfo{
		IP:     ip,
		Domain: ip,
		Name:   removeConnectorNamePrefix(name),
	}); err != nil {
		return err
	}

	return nil
}
