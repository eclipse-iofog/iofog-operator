package kog

import (
	"context"
	"fmt"
	"strings"

	k8sv1alpha2 "github.com/eclipse-iofog/iofog-operator/pkg/apis/k8s/v1alpha2"
	k8sclient "github.com/eclipse-iofog/iofog-operator/pkg/controller/client"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcileKog) reconcileIofogConnectors(kog *k8sv1alpha2.Kog) error {

	// Find the current state to compare against requested state
	depList := &appsv1.DeploymentList{}
	if err := r.client.List(context.Background(), &client.ListOptions{}, depList); err != nil {
		return err
	}
	// Determine which connectors to create and delete
	createConnectors := make(map[string]bool)
	deleteConnectors := make(map[string]bool)
	for _, connector := range kog.Spec.Connectors.Instances {
		name := prefixConnectorName(connector.Name)
		createConnectors[name] = true
		deleteConnectors[name] = false
	}
	for _, dep := range depList.Items {
		if strings.Contains(dep.ObjectMeta.Name, getConnectorNamePrefix()) {
			createConnectors[dep.ObjectMeta.Name] = false
			if _, exists := deleteConnectors[dep.ObjectMeta.Name]; !exists {
				deleteConnectors[dep.ObjectMeta.Name] = true
			}
		}
	}

	// Delete connectors
	for k, v := range deleteConnectors {
		if v {
			r.deleteConnector(kog, k)
		}
	}

	// Create connectors
	for k, v := range createConnectors {
		if v {
			r.createConnector(kog, k)
		}
	}

	// Update existing Connector deployments (e.g. image change)
	for k, v := range deleteConnectors {
		if !v {
			ms := newConnectorMicroservice(kog.Spec.Connectors.Image)
			ms.name = k
			// Deployment
			if err := r.createDeployment(kog, ms); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *ReconcileKog) reconcileIofogController(kog *k8sv1alpha2.Kog) error {
	// Configure
	ms := newControllerMicroservice(kog.Spec.ControlPlane.ControllerReplicaCount, kog.Spec.ControlPlane.ControllerImage, &kog.Spec.ControlPlane.Database)
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

	// Connect to cluster
	k8sClient, err := k8sclient.NewClient()
	if err != nil {
		return err
	}

	// Wait for Pods
	if err = k8sClient.WaitForPod(kog.ObjectMeta.Namespace, ms.name, 120); err != nil {
		return err
	}

	// Wait for Service
	_, err = k8sClient.WaitForService(kog.ObjectMeta.Namespace, ms.name, 240)
	if err != nil {
		return err
	}
	if err = r.waitForControllerAPI(); err != nil {
		return err
	}

	// Set up user
	if err = r.createIofogUser(&kog.Spec.ControlPlane.IofogUser); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileKog) reconcileIofogKubelet(kog *k8sv1alpha2.Kog) error {
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
