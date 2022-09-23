/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/apps/v3"
)

// ApplicationReconciler reconciles a Application object
type ApplicationReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=iofog.org,resources=applications,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iofog.org,resources=applications/status,verbs=get;update;patch

func (r *ApplicationReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("application", request.NamespacedName)

	instance := &appsv3.Application{}
	err := r.Client.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	found := &appsv1.Deployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		dep, err := r.deploymentForApp(instance)
		if err != nil {
			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

		err = r.Client.Create(ctx, dep)
		if err != nil {
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")

		return ctrl.Result{}, err
	}

	count := instance.Spec.Replicas
	log.Info("Scaling", "Current count: ", *found.Spec.Replicas)
	log.Info("Scaling", "Desired count: ", count)

	if *found.Spec.Replicas != count {
		found.Spec.Replicas = &count
		err = r.Client.Update(ctx, found)
		if err != nil {
			log.Error(err, "Failed to update Deployment", "Deployment.Namespace", instance.Namespace, "Deployment.Name", instance.Name)

			return ctrl.Result{}, err
		}

		return ctrl.Result{Requeue: true}, nil
	}

	podList := &corev1.PodList{}
	err = r.Client.List(ctx, podList)
	if err != nil {
		log.Error(err, "Failed to list pods", "Deployment.Namespace", instance.Namespace, "Deployment.Name", instance.Name)

		return ctrl.Result{}, err
	}

	podNames := getPodNames(podList.Items)

	if !reflect.DeepEqual(podNames, instance.Status.PodNames) {
		instance.Status.PodNames = podNames
		err := r.Client.Update(ctx, instance)
		if err != nil {
			log.Error(err, "failed to update node status", "Deployment.Namespace", instance.Namespace, "Deployment.Name", instance.Name)

			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ApplicationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv3.Application{}).
		Complete(r)
}

func getPodNames(pods []corev1.Pod) []string {
	podNames := make([]string, len(pods))
	for i := range pods {
		podNames[i] = pods[i].Name
	}

	return podNames
}

func labelsForIOFog(name string) map[string]string {
	return map[string]string{
		"app": name,
	}
}

func (r *ApplicationReconciler) deploymentForApp(app *appsv3.Application) (*appsv1.Deployment, error) {
	labels := labelsForIOFog(app.Name)

	microservices, err := json.Marshal(app.Spec.Microservices)
	if err != nil {
		return nil, err
	}

	routes, err := json.Marshal(app.Spec.Routes)
	if err != nil {
		return nil, err
	}

	annotations := map[string]string{
		"microservices": string(microservices),
		"routes":        string(routes),
	}

	containers := make([]corev1.Container, len(app.Spec.Microservices))

	for i := range app.Spec.Microservices {
		microservice := &app.Spec.Microservices[i]
		container := corev1.Container{
			Name:  microservice.Name,
			Image: fmt.Sprintf("%s, %s", microservice.Images.X86, microservice.Images.ARM),
		}
		containers[i] = container
	}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.ObjectMeta.Name,
			Namespace: app.ObjectMeta.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &app.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:      "resource-type",
							Operator: corev1.TolerationOpEqual,
							Value:    "iofog-custom-resource",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
					Affinity: &corev1.Affinity{
						NodeAffinity: &corev1.NodeAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
								NodeSelectorTerms: []corev1.NodeSelectorTerm{
									{
										MatchExpressions: []corev1.NodeSelectorRequirement{
											{
												Key:      "type",
												Operator: corev1.NodeSelectorOpIn,
												Values:   []string{"iofog-kubelet"},
											},
										},
									},
								},
							},
						},
						PodAntiAffinity: &corev1.PodAntiAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchExpressions: []metav1.LabelSelectorRequirement{
											{
												Key:      "app",
												Operator: metav1.LabelSelectorOpIn,
												Values:   []string{app.ObjectMeta.Name},
											},
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Containers: containers,
				},
			},
		},
	}

	return dep, controllerutil.SetControllerReference(app, dep, r.Scheme)
}
