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

package controllers

import (
	"errors"
	"strconv"
	"strings"

	// "k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
	"github.com/eclipse-iofog/iofog-operator/v3/internal/util"
)

const (
	routerName                      = "router"
	controllerName                  = "controller"
	controllerCredentialsSecretName = "controller-credentials"
	emailSecretKey                  = "email"
	passwordSecretKey               = "password"
)

type service struct {
	name             string
	loadBalancerAddr string
	trafficPolicy    string
	serviceType      string
	ports            []int
}

type microservice struct {
	name                  string
	services              []service
	imagePullSecret       string
	replicas              int32
	containers            []container
	labels                map[string]string
	annotations           map[string]string
	secrets               []corev1.Secret
	volumes               []corev1.Volume
	rbacRules             []rbacv1.PolicyRule
	mustRecreateOnRollout bool
	availableDelay        int32
}

type container struct {
	name            string
	image           string
	imagePullPolicy string
	args            []string
	readinessProbe  *corev1.Probe
	env             []corev1.EnvVar
	command         []string
	ports           []corev1.ContainerPort
	resources       corev1.ResourceRequirements
	volumeMounts    []corev1.VolumeMount
}

type controllerMicroserviceConfig struct {
	replicas          int32
	image             string
	imagePullSecret   string
	serviceType       string
	loadBalancerAddr  string
	db                *cpv3.Database
	proxyImage        string
	routerImage       string
	portProvider      string
	portAllocatorHost string
	ecn               string
	pidBaseDir        string
	ecnViewerPort     int
}

func filterControllerConfig(cfg *controllerMicroserviceConfig) {
	if cfg.replicas == 0 {
		cfg.replicas = 1
	}
	if cfg.image == "" {
		cfg.image = util.GetControllerImage()
	}
	if cfg.serviceType == "" {
		cfg.serviceType = string(corev1.ServiceTypeLoadBalancer)
	}
	if cfg.ecnViewerPort == 0 {
		cfg.ecnViewerPort = 80
	}
	if cfg.pidBaseDir == "" {
		cfg.pidBaseDir = "/tmp"
	}
}

func getControllerPort(msvc *microservice) (int, error) {
	if len(msvc.services) == 0 || len(msvc.services[0].ports) == 0 {
		return 0, errors.New("controller microservice does not have requisite ports")
	}
	return msvc.services[0].ports[0], nil
}

func newControllerMicroservice(cfg *controllerMicroserviceConfig) *microservice {
	filterControllerConfig(cfg)
	msvc := &microservice{
		availableDelay: 5,
		name:           "controller",
		labels: map[string]string{
			"name": "controller",
		},
		imagePullSecret: cfg.imagePullSecret,
		replicas:        cfg.replicas,
		services: []service{
			{
				name:             "controller",
				serviceType:      cfg.serviceType,
				trafficPolicy:    getTrafficPolicy(cfg.serviceType),
				loadBalancerAddr: cfg.loadBalancerAddr,
				ports: []int{
					51121,
					80,
				},
			},
		},
		containers: []container{
			{
				name:            "controller",
				image:           cfg.image,
				imagePullPolicy: "Always",
				readinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path: "/api/v3/status",
							Port: intstr.FromInt(51121),
						},
					},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
				},
				env: []corev1.EnvVar{
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
					{
						Name:  "MSVC_PORT_PROVIDER",
						Value: cfg.portProvider,
					},
					{
						Name:  "SystemImages_Proxy_1",
						Value: cfg.proxyImage,
					},
					{
						Name:  "SystemImages_Proxy_2",
						Value: util.TransformImageToARM(cfg.proxyImage),
					},
					{
						Name:  "SystemImages_Router_1",
						Value: cfg.routerImage,
					},
					{
						Name:  "SystemImages_Router_2",
						Value: util.TransformImageToARM(cfg.routerImage),
					},
					{
						Name:  "PORT_ALLOC_ADDRESS",
						Value: cfg.portAllocatorHost,
					},
					{
						Name:  "ECN_NAME",
						Value: cfg.ecn,
					},
					{
						Name:  "PID_BASE",
						Value: cfg.pidBaseDir,
					},
					{
						Name:  "VIEWER_PORT",
						Value: strconv.Itoa(cfg.ecnViewerPort),
					},
				},
				// resources: corev1.ResourceRequirements{
				// 	Limits: corev1.ResourceList{
				// 		"cpu":    resource.MustParse("1800m"),
				// 		"memory": resource.MustParse("3Gi"),
				// 	},
				// 	Requests: corev1.ResourceList{
				// 		"cpu":    resource.MustParse("400m"),
				// 		"memory": resource.MustParse("1Gi"),
				// 	},
				// },
			},
		},
	}
	// Add PVC details if no external DB provided
	if cfg.db.Host == "" {
		msvc.mustRecreateOnRollout = true
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

type portManagerConfig struct {
	image            string
	proxyImage       string
	httpProxyAddress string
	tcpProxyAddress  string
	watchNamespace   string
	userEmail        string
	userPass         string
}

func filterPortManagerConfig(cfg *portManagerConfig) {
	if cfg.image == "" {
		cfg.image = util.GetPortManagerImage()
	}
	if cfg.proxyImage == "" {
		cfg.proxyImage = util.GetProxyImage()
	}
}

func newPortManagerMicroservice(cfg *portManagerConfig) *microservice {
	filterPortManagerConfig(cfg)
	return &microservice{
		mustRecreateOnRollout: true,
		name:                  "port-manager",
		labels: map[string]string{
			"name": "port-manager",
		},
		replicas: 1,
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				APIGroups: []string{"", "apps"},
				Resources: []string{"deployments", "services", "pods", "configmaps"},
			},
		},
		secrets: []corev1.Secret{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: controllerCredentialsSecretName,
				},
				StringData: map[string]string{
					emailSecretKey:    cfg.userEmail,
					passwordSecretKey: cfg.userPass,
				},
			},
		},
		containers: []container{
			{
				name:            "port-manager",
				image:           cfg.image,
				imagePullPolicy: "Always",
				readinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						Exec: &corev1.ExecAction{
							Command: []string{
								"stat",
								"/tmp/operator-sdk-ready",
							},
						},
					},
					InitialDelaySeconds: 4,
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
				},
				// resources: corev1.ResourceRequirements{
				// 	 Limits: corev1.ResourceList{
				// 	 	"cpu":    resource.MustParse("200m"),
				// 	 	"memory": resource.MustParse("1Gi"),
				// 	 },
				// 	 Requests: corev1.ResourceList{
				// 	 	"cpu":    resource.MustParse("50m"),
				// 	 	"memory": resource.MustParse("200Mi"),
				// 	 },
				// },
				env: []corev1.EnvVar{
					{
						Name:  "WATCH_NAMESPACE",
						Value: cfg.watchNamespace,
					},
					{
						Name: "POD_NAME",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
					{
						Name:  "OPERATOR_NAME",
						Value: "port-manager",
					},
					{
						Name: "IOFOG_USER_EMAIL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerCredentialsSecretName,
								},
								Key: passwordSecretKey,
							},
						},
					},
					{
						Name: "IOFOG_USER_PASS",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerCredentialsSecretName,
								},
								Key: emailSecretKey,
							},
						},
					},
					{
						Name:  "PROXY_IMAGE",
						Value: cfg.proxyImage,
					},
					{
						Name:  "HTTP_PROXY_ADDRESS",
						Value: cfg.httpProxyAddress,
					},
					{
						Name:  "TCP_PROXY_ADDRESS",
						Value: cfg.tcpProxyAddress,
					},
					{
						Name:  "ROUTER_ADDRESS",
						Value: routerName,
					},
				},
			},
		},
	}
}

type routerMicroserviceConfig struct {
	image           string
	serviceType     string
	volumeMountPath string
}

func filterRouterConfig(cfg routerMicroserviceConfig) routerMicroserviceConfig {
	if cfg.image == "" {
		cfg.image = util.GetRouterImage()
	}
	if cfg.serviceType == "" {
		cfg.serviceType = string(corev1.ServiceTypeLoadBalancer)
	}
	return cfg
}

func newRouterMicroservice(cfg routerMicroserviceConfig) *microservice {
	cfg = filterRouterConfig(cfg)
	return &microservice{
		name: routerName,
		labels: map[string]string{
			"name":                 routerName,
			"application":          "interior-router",
			"skupper.io/component": "router",
		},
		annotations: map[string]string{
			"prometheus.io/port":   "9090",
			"prometheus.io/scrape": "true",
		},
		services: []service{
			{
				name:          "router",
				serviceType:   cfg.serviceType,
				trafficPolicy: getTrafficPolicy(cfg.serviceType),
				ports: []int{
					router.MessagePort,
					router.InteriorPort,
					router.EdgePort,
				},
			},
		},
		replicas: 1,
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
		volumes: []corev1.Volume{

			{
				Name: routerName + "-internal",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "router-internal",
					},
				},
			},
			{
				Name: routerName + "-amqps",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: routerName + "-amqps",
					},
				},
			},
		},
		containers: []container{
			{
				name:            routerName,
				image:           cfg.image,
				imagePullPolicy: "Always",
				command: []string{
					"/qpid-dispatch/launch.sh",
				},
				readinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Port: intstr.FromInt(9090),
							Path: "/healthz",
						},
					},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
				},
				env: []corev1.EnvVar{
					{
						Name:  "APPLICATION_NAME",
						Value: routerName,
					},
					{
						Name:  "QDROUTERD_AUTO_MESH_DISCOVERY",
						Value: "QUERY",
					},
					{
						Name:  "QDROUTERD_CONF",
						Value: router.GetConfig(),
					},
					{
						Name: "POD_NAMESPACE",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
					{
						Name: "POD_IP",
						ValueFrom: &corev1.EnvVarSource{
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "status.podIP",
							},
						},
					},
				},
				volumeMounts: []corev1.VolumeMount{
					{
						Name:      routerName + "-internal",
						MountPath: cfg.volumeMountPath + "/router-internal",
					},
					{
						Name:      routerName + "-amqps",
						MountPath: cfg.volumeMountPath + "/" + routerName + "-amqps",
					},
				},
				// resources: corev1.ResourceRequirements{
				// 	 Limits: corev1.ResourceList{
				// 	 	"cpu":    resource.MustParse("200m"),
				// 	 	"memory": resource.MustParse("1Gi"),
				// 	 },
				// 	 Requests: corev1.ResourceList{
				// 	 	"cpu":    resource.MustParse("50m"),
				// 	 	"memory": resource.MustParse("200Mi"),
				// 	 },
				// },
			},
		},
	}
}

func getTrafficPolicy(serviceType string) string {
	if strings.EqualFold(serviceType, string(corev1.ServiceTypeLoadBalancer)) {
		return string(corev1.ServiceExternalTrafficPolicyTypeLocal)
	}
	return ""
}
