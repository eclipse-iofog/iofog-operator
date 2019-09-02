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

package kog

import (
	"fmt"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type microservice struct {
	name       string
	IP         string
	ports      []int
	replicas   int32
	containers []container
}

type container struct {
	name            string
	image           string
	imagePullPolicy string
	args            []string
	readinessProbe  *v1.Probe
	env             []v1.EnvVar
	command         []string
	ports           []v1.ContainerPort
}

func newControllerMicroservice(replicas int32, image string) *microservice {
	if replicas == 0 {
		replicas = 1
	}
	if image == "" {
		image = "iofog/controller:1.2.1"
	}
	return &microservice{
		name:     "controller",
		ports:    []int{51121},
		replicas: replicas,
		containers: []container{
			{
				name:            "controller",
				image:           image,
				imagePullPolicy: "Always",
				readinessProbe: &v1.Probe{
					Handler: v1.Handler{
						HTTPGet: &v1.HTTPGetAction{
							Path: "/api/v3/status",
							Port: intstr.FromInt(51121),
						},
					},
					InitialDelaySeconds: 1,
					PeriodSeconds:       4,
					FailureThreshold:    3,
				},
			},
		},
	}
}

func newConnectorMicroservice(image string) *microservice {
	if image == "" {
		image = "iofog/connector:1.2.0"
	}
	return &microservice{
		name: "connector",
		ports: []int{
			8080,
			6000, 6001, 6002, 6003, 6004, 6005, 6006, 6007, 6008, 6009,
			6010, 6011, 6012, 6013, 6014, 6015, 6016, 6017, 6018, 6019,
			6020, 6021, 6022, 6023, 6024, 6025, 6026, 6027, 6028, 6029,
			6030, 6031, 6032, 6033, 6034, 6035, 6036, 6037, 6038, 6039,
			6040, 6041, 6042, 6043, 6044, 6045, 6046, 6047, 6048, 6049,
			6050,
		},
		replicas: 1,
		containers: []container{
			{
				name:            "connector",
				image:           image,
				imagePullPolicy: "Always",
			},
		},
	}
}

func newKubeletMicroservice(image, namespace, token, controllerEndpoint string) *microservice {
	if image == "" {
		image = "iofog/iofog-kubelet:1.2.0"
	}
	return &microservice{
		name:     "kubelet",
		ports:    []int{60000},
		replicas: 1,
		containers: []container{
			{
				name:            "kubelet",
				image:           image,
				imagePullPolicy: "Always",
				args: []string{
					"--namespace",
					namespace,
					"--iofog-token",
					token,
					"--iofog-url",
					fmt.Sprintf("http://%s", controllerEndpoint),
				},
			},
		},
	}
}
