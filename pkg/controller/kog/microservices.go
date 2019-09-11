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
	"errors"
	"fmt"
	k8sv1alpha2 "github.com/eclipse-iofog/iofog-operator/pkg/apis/k8s/v1alpha2"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"strings"
)

func getConnectorNamePrefix() string {
	return "connector-"
}

func prefixConnectorName(name string) string {
	return "connector-" + name
}

func removeConnectorNamePrefix(name string) string {
	pos := strings.Index(name, "-")
	if pos == -1 || pos >= len(name)-1 {
		return name
	}
	return name[pos+1:]
}

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

func newControllerMicroservice(replicas int32, image string, db *k8sv1alpha2.Database) *microservice {
	if replicas == 0 {
		replicas = 1
	}
	if image == "" {
		image = "iofog/controller:1.2.1"
	}
	return &microservice{
		name: "controller",
		ports: []int{
			51121,
			80,
		},
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
				env: []v1.EnvVar{
					{
						Name:  "DB_PROVIDER",
						Value: db.Provider,
					},
					{
						Name:  "DB_NAME",
						Value: db.DatabaseName,
					},
					{
						Name:  "DB_USERNAME",
						Value: db.User,
					},
					{
						Name:  "DB_PASSWORD",
						Value: db.Password,
					},
					{
						Name:  "DB_HOST",
						Value: db.Host,
					},
					{
						Name:  "DB_PORT",
						Value: strconv.Itoa(db.Port),
					},
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

func getKubeletToken(containers []corev1.Container) (token string, err error) {
	if len(containers) != 1 {
		err = errors.New(fmt.Sprintf("Expected 1 container in Kubelet deployment config. Found %d", len(containers)))
		return
	}
	if len(containers[0].Args) != 6 {
		err = errors.New(fmt.Sprintf("Expected 6 args in Kubelet deployment config. Found %d", len(containers[0].Args)))
		return
	}
	token = containers[0].Args[3]
	return
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
