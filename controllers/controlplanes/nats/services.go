/*
 *  *******************************************************************************
 *  * Copyright (c) 2023 Contributors to the Eclipse ioFog Project
 *  *
 *  * This program and the accompanying materials are made available under the
 *  * terms of the Eclipse Public License v. 2.0 which is available at
 *  * http://www.eclipse.org/legal/epl-2.0
 *  *
 *  * SPDX-License-Identifier: EPL-2.0
 *  *******************************************************************************
 *
 */

package nats

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// NewNatsHeadlessService creates the headless Service for the NATS StatefulSet (all ports).
func NewNatsHeadlessService(namespace string, labels map[string]string) *corev1.Service {
	ports := []corev1.ServicePort{
		{Name: "cluster", Port: int32(DefaultClusterPort), TargetPort: intstr.FromInt(DefaultClusterPort)},
		{Name: "leaf", Port: int32(DefaultLeafPort), TargetPort: intstr.FromInt(DefaultLeafPort)},
		{Name: "mqtt", Port: int32(DefaultMqttPort), TargetPort: intstr.FromInt(DefaultMqttPort)},
		{Name: "client", Port: int32(DefaultServerPort), TargetPort: intstr.FromInt(DefaultServerPort)},
		{Name: "monitor", Port: int32(DefaultHttpPort), TargetPort: intstr.FromInt(DefaultHttpPort)},
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HeadlessServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: corev1.ClusterIPNone,
			Selector:  labels,
			Ports:     ports,
		},
	}
}

// NewNatsClientService creates the client-facing Service for NATS (cluster, leaf, mqtt).
func NewNatsClientService(namespace string, labels map[string]string, serviceType corev1.ServiceType, annotations map[string]string) *corev1.Service {
	ports := []corev1.ServicePort{
		{Name: "cluster", Port: int32(DefaultClusterPort), TargetPort: intstr.FromInt(DefaultClusterPort)},
		{Name: "leaf", Port: int32(DefaultLeafPort), TargetPort: intstr.FromInt(DefaultLeafPort)},
		{Name: "mqtt", Port: int32(DefaultMqttPort), TargetPort: intstr.FromInt(DefaultMqttPort)},
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ClientServiceName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:     serviceType,
			Selector: labels,
			Ports:    ports,
		},
	}
}

// NewNatsServerService creates the nats-server Service (client and monitor ports only, ClusterIP).
func NewNatsServerService(namespace string, labels map[string]string) *corev1.Service {
	ports := []corev1.ServicePort{
		{Name: "client", Port: int32(DefaultServerPort), TargetPort: intstr.FromInt(DefaultServerPort)},
		{Name: "monitor", Port: int32(DefaultHttpPort), TargetPort: intstr.FromInt(DefaultHttpPort)},
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServerServiceName,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports:    ports,
		},
	}
}
