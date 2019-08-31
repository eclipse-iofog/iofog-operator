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
	"github.com/eclipse-iofog/iofogctl/pkg/iofog"
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

var controllerMicroservice = microservice{
	name:     "controller",
	ports:    []int{iofog.ControllerPort},
	replicas: 1,
	containers: []container{
		{
			name:            "controller",
			image:           "iofog/controller:" + "1.2.1",
			imagePullPolicy: "Always",
			readinessProbe: &v1.Probe{
				Handler: v1.Handler{
					HTTPGet: &v1.HTTPGetAction{
						Path: "/api/v3/status",
						Port: intstr.FromInt(iofog.ControllerPort),
					},
				},
				InitialDelaySeconds: 1,
				PeriodSeconds:       4,
				FailureThreshold:    3,
			},
		},
	},
}

var schedulerMicroservice = microservice{
	name:     "scheduler",
	replicas: 1,
	containers: []container{
		{
			name:            "scheduler",
			image:           "iofog/iofog-scheduler:1.2.0",
			imagePullPolicy: "Always",
		},
	},
}

var kubeletMicroservice = microservice{
	name:     "kubelet",
	ports:    []int{60000},
	replicas: 1,
	containers: []container{
		{
			name:            "kubelet",
			image:           "iofog/iofog-kubelet:1.2.0",
			imagePullPolicy: "Always",
			args: []string{
				"--namespace",
				"",
				"--iofog-token",
				"",
				"--iofog-url",
				"",
			},
		},
	},
}
