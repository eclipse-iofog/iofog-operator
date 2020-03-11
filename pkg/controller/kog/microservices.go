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
	"strconv"
	"strings"

	iofogv1 "github.com/eclipse-iofog/iofog-operator/pkg/apis/iofog/v1"
	"github.com/eclipse-iofog/iofog-operator/pkg/controller/kog/skupper"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	skupperName = "skupper"
)

func removeConnectorNamePrefix(name string) string {
	pos := strings.Index(name, "-")
	if pos == -1 || pos >= len(name)-1 {
		return name
	}
	return name[pos+1:]
}

type microservice struct {
	name            string
	loadBalancerIP  string
	serviceType     string
	trafficPolicy   string
	imagePullSecret string
	ports           []int
	replicas        int32
	containers      []container
	labels          map[string]string
	annotations     map[string]string
	secrets         []v1.Secret
	volumes         []v1.Volume
	rbacRules       []rbacv1.PolicyRule
}

type container struct {
	name            string
	image           string
	imagePullPolicy string
	args            []string
	livenessProbe   *v1.Probe
	readinessProbe  *v1.Probe
	env             []v1.EnvVar
	command         []string
	ports           []v1.ContainerPort
	resources       v1.ResourceRequirements
	volumeMounts    []v1.VolumeMount
}

type controllerMicroserviceConfig struct {
	replicas        int32
	image           string
	imagePullSecret string
	serviceType     string
	loadBalancerIP  string
	db              *iofogv1.Database
}

func newControllerMicroservice(cfg controllerMicroserviceConfig) *microservice {
	if cfg.replicas == 0 {
		cfg.replicas = 1
	}
	msvc := &microservice{
		name: "controller",
		labels: map[string]string{
			"name": "controller",
		},
		ports: []int{
			51121,
			80,
		},
		imagePullSecret: cfg.imagePullSecret,
		replicas:        cfg.replicas,
		serviceType:     cfg.serviceType,
		trafficPolicy:   getTrafficPolicy(cfg.serviceType),
		loadBalancerIP:  cfg.loadBalancerIP,
		containers: []container{
			{
				name:            "controller",
				image:           cfg.image,
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
						Value: cfg.db.Provider,
					},
					{
						Name:  "DB_NAME",
						Value: cfg.db.DatabaseName,
					},
					{
						Name:  "DB_USERNAME",
						Value: cfg.db.User,
					},
					{
						Name:  "DB_PASSWORD",
						Value: cfg.db.Password,
					},
					{
						Name:  "DB_HOST",
						Value: cfg.db.Host,
					},
					{
						Name:  "DB_PORT",
						Value: strconv.Itoa(cfg.db.Port),
					},
				},
				resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"cpu":    resource.MustParse("1800m"),
						"memory": resource.MustParse("3Gi"),
					},
					Requests: v1.ResourceList{
						"cpu":    resource.MustParse("400m"),
						"memory": resource.MustParse("1Gi"),
					},
				},
			},
		},
	}
	// Add PVC details if no external DB provided
	if cfg.db.Host == "" {
		msvc.volumes = []corev1.Volume{
			{
				Name: "controller-sqlite",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "controller-sqlite",
						ReadOnly:  false,
					},
				},
			},
		}
		msvc.containers[0].volumeMounts = []corev1.VolumeMount{
			{
				Name:      "controller-sqlite",
				MountPath: "/usr/local/lib/node_modules/iofogcontroller/src/data/sqlite_files/",
				SubPath:   "prod_database.sqlite",
			},
		}
	}
	return msvc
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
	return &microservice{
		name: "kubelet",
		labels: map[string]string{
			"name": "kubelet",
		},
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
				resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"cpu":    resource.MustParse("200m"),
						"memory": resource.MustParse("1Gi"),
					},
					Requests: v1.ResourceList{
						"cpu":    resource.MustParse("50m"),
						"memory": resource.MustParse("200Mi"),
					},
				},
			},
		},
	}
}

func newPortManagerMicroservice(image, proxyImage, watchNamespace, iofogUserEmail, iofogUserPass string) *microservice {
	return &microservice{
		name: "port-manager",
		labels: map[string]string{
			"name": "port-manager",
		},
		replicas: 1,
		containers: []container{
			{
				name:            "port-manager",
				image:           image,
				imagePullPolicy: "Always",
				readinessProbe: &v1.Probe{
					Handler: v1.Handler{
						Exec: &v1.ExecAction{
							Command: []string{
								"stat",
								"/tmp/operator-sdk-ready",
							},
						},
					},
					InitialDelaySeconds: 4,
					PeriodSeconds:       10,
					FailureThreshold:    1,
				},
				resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"cpu":    resource.MustParse("200m"),
						"memory": resource.MustParse("1Gi"),
					},
					Requests: v1.ResourceList{
						"cpu":    resource.MustParse("50m"),
						"memory": resource.MustParse("200Mi"),
					},
				},
				env: []v1.EnvVar{
					{
						Name:  "WATCH_NAMESPACE",
						Value: watchNamespace,
					},
					{
						Name: "POD_NAME",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &v1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name:  "OPERATOR_NAME",
						Value: "port-manager",
					},
					{
						Name:  "IOFOG_USER_EMAIL",
						Value: iofogUserEmail,
					},
					{
						Name:  "IOFOG_USER_PASS",
						Value: iofogUserPass,
					},
					{
						Name:  "PROXY_IMAGE",
						Value: proxyImage,
					},
					{
						Name:  "ROUTER_ADDRESS",
						Value: skupperName,
					},
				},
			},
		},
	}
}

func newSkupperMicroservice(image, volumeMountPath string) *microservice {
	return &microservice{
		name: skupperName,
		labels: map[string]string{
			"name":                 skupperName,
			"application":          "interior-router",
			"skupper.io/component": "router",
		},
		annotations: map[string]string{
			"prometheus.io/port":   "9090",
			"prometheus.io/scrape": "true",
		},
		ports: []int{
			skupper.MessagePort,
			skupper.HTTPPort,
			skupper.InteriorPort,
			skupper.EdgePort,
		},
		replicas:      1,
		serviceType:   "LoadBalancer",
		trafficPolicy: "Local",
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
		volumes: []v1.Volume{
			{
				Name: "skupper-internal",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "skupper-internal",
					},
				},
			},
			{
				Name: "skupper-amqps",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "skupper-amqps",
					},
				},
			},
		},
		containers: []container{
			{
				name:            skupperName,
				image:           image,
				imagePullPolicy: "Always",
				livenessProbe: &corev1.Probe{
					InitialDelaySeconds: 60,
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.FromInt(9090),
							Path: "/healthz",
						},
					},
				},
				env: []v1.EnvVar{
					{
						Name:  "APPLICATION_NAME",
						Value: "skupper-router",
					},
					{
						Name:  "QDROUTERD_AUTO_MESH_DISCOVERY",
						Value: "QUERY",
					},
					{
						Name:  "QDROUTERD_CONF",
						Value: skupper.GetRouterConfig(),
					},
					{
						Name: "POD_NAMESPACE",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
					{
						Name: "POD_IP",
						ValueFrom: &v1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				volumeMounts: []v1.VolumeMount{
					{
						Name:      "skupper-internal",
						MountPath: volumeMountPath + "/skupper-internal",
					},
					{
						Name:      "skupper-amqps",
						MountPath: volumeMountPath + "/skupper-amqps",
					},
				},
				resources: v1.ResourceRequirements{
					Limits: v1.ResourceList{
						"cpu":    resource.MustParse("200m"),
						"memory": resource.MustParse("1Gi"),
					},
					Requests: v1.ResourceList{
						"cpu":    resource.MustParse("50m"),
						"memory": resource.MustParse("200Mi"),
					},
				},
			},
		},
	}
}

func getTrafficPolicy(serviceType string) string {
	if serviceType == string(corev1.ServiceTypeLoadBalancer) {
		return string(corev1.ServiceExternalTrafficPolicyTypeLocal)
	}
	return string(corev1.ServiceExternalTrafficPolicyTypeCluster)
}
