/*
 *  *******************************************************************************
 *  * Copyright (c) 2019 Edgeworx, Inc.
 *  *
 *  * This program and the accompanying materials are made available under the
 *  * terms of the Eclipse Public License v. 2.0 which is available at
 *  * http://www.eclipse.org/legal/epl-2.0
 *  *
 *  * SPDX-License-Identifier: EPL-2.0
 *  *******************************************************************************
 *
 */

package controlplane

import (
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func newServices(namespace string, ms *microservice) (svcs []*corev1.Service) {
	for _, msvcSvc := range ms.services {
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      msvcSvc.name,
				Namespace: namespace,
				Labels:    ms.labels,
			},
			Spec: corev1.ServiceSpec{
				Type:                  corev1.ServiceType(msvcSvc.serviceType),
				ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyType(msvcSvc.trafficPolicy),
				LoadBalancerIP:        msvcSvc.loadBalancerAddr,
				Selector:              ms.labels,
			},
		}
		// Add ports
		for idx, port := range msvcSvc.ports {
			svcPort := corev1.ServicePort{
				Name:       msvcSvc.name + strconv.Itoa(idx),
				Port:       int32(port),
				TargetPort: intstr.FromInt(port),
				Protocol:   corev1.Protocol("TCP"),
			}
			svc.Spec.Ports = append(svc.Spec.Ports, svcPort)
		}
		svcs = append(svcs, svc)
	}
	return svcs
}

func newDeployment(namespace string, ms *microservice) *appsv1.Deployment {
	maxUnavailable := intstr.FromInt(0)
	maxSurge := intstr.FromInt(1)
	strategy := appsv1.DeploymentStrategy{
		Type: appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{
			MaxUnavailable: &maxUnavailable,
			MaxSurge:       &maxSurge,
		},
	}
	if ms.mustRecreateOnRollout {
		strategy = appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		}
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.name,
			Namespace: namespace,
			Labels:    ms.labels,
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds: ms.availableDelay,
			Replicas:        &ms.replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ms.labels,
			},
			Strategy: strategy,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ms.labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ms.name,
					Volumes:            ms.volumes,
				},
			},
		},
	}
	containers := &dep.Spec.Template.Spec.Containers
	for idx := range ms.containers {
		msCont := &ms.containers[idx]
		cont := corev1.Container{
			Name:            msCont.name,
			Image:           msCont.image,
			Command:         msCont.command,
			Args:            msCont.args,
			Ports:           msCont.ports,
			Env:             msCont.env,
			Resources:       msCont.resources,
			ReadinessProbe:  msCont.readinessProbe,
			VolumeMounts:    msCont.volumeMounts,
			ImagePullPolicy: corev1.PullPolicy(msCont.imagePullPolicy),
		}
		*containers = append(*containers, cont)
	}
	return dep
}

func newServiceAccount(namespace string, ms *microservice) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.name,
			Namespace: namespace,
		},
	}
}

func newRoleBinding(namespace string, ms *microservice) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.name,
			Namespace: namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind: "ServiceAccount",
				Name: ms.name,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     ms.name,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}
}

func newRole(namespace string, ms *microservice) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ms.name,
			Namespace: namespace,
		},
		Rules: ms.rbacRules,
	}
}
