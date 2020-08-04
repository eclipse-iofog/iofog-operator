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
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/eclipse-iofog/iofog-operator/v2/internal/util"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/apis/iofog"
	"github.com/eclipse-iofog/iofog-operator/v2/pkg/controller/controlplane/router"
	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	routerName = "router"
)

func removeConnectorNamePrefix(name string) string {
	pos := strings.Index(name, "-")
	if pos == -1 || pos >= len(name)-1 {
		return name
	}
	return name[pos+1:]
}

type microservice struct {
	name                  string
	loadBalancerAddr      string
	serviceType           string
	trafficPolicy         string
	imagePullSecret       string
	ports                 []int
	replicas              int32
	containers            []container
	labels                map[string]string
	annotations           map[string]string
	secrets               []v1.Secret
	volumes               []v1.Volume
	rbacRules             []rbacv1.PolicyRule
	mustRecreateOnRollout bool
	availableDelay        int32
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
	replicas         int32
	image            string
	imagePullSecret  string
	serviceType      string
	loadBalancerAddr string
	db               *iofog.Database
	proxyImage       string
	routerImage      string
	portProvider     string
	httpPortAddr     string
	tcpPortAddr      string
	tcpAllocatorHost string
	tcpAllocatorPort int
	ecnId            int
}

func filterControllerConfig(cfg controllerMicroserviceConfig) controllerMicroserviceConfig {
	if cfg.replicas == 0 {
		cfg.replicas = 1
	}
	if cfg.image == "" {
		cfg.image = util.GetControllerImage()
	}
	if cfg.serviceType == "" {
		cfg.serviceType = string(corev1.ServiceTypeLoadBalancer)
	}
	if cfg.httpPortAddr != "" && cfg.tcpPortAddr != "" {
		cfg.portProvider = "caas"
	}
	return cfg
}

func newControllerMicroservice(cfg controllerMicroserviceConfig) *microservice {
	cfg = filterControllerConfig(cfg)
	msvc := &microservice{
		availableDelay: 5,
		name:           "controller",
		labels: map[string]string{
			"name": "controller",
		},
		ports: []int{
			51121,
			80,
		},
		imagePullSecret:  cfg.imagePullSecret,
		replicas:         cfg.replicas,
		serviceType:      cfg.serviceType,
		trafficPolicy:    getTrafficPolicy(cfg.serviceType),
		loadBalancerAddr: cfg.loadBalancerAddr,
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
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
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
					{
						Name:  "MSVC_PORT_PROVIDER",
						Value: cfg.portProvider,
					},
					{
						Name:  "TCP_PORT_ADDR",
						Value: cfg.tcpPortAddr,
					},
					{
						Name:  "HTTP_PORT_ADDR",
						Value: cfg.httpPortAddr,
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
						Name:  "TCP_ALLOC_ADDRESS",
						Value: fmt.Sprintf("%s:%d", cfg.tcpAllocatorHost, cfg.tcpAllocatorPort),
					},
					{
						Name:  "ECN_ID",
						Value: strconv.Itoa(cfg.ecnId),
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
		image = util.GetKubeletImage()
	}
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

type portManagerConfig struct {
	image            string
	proxyImage       string
	httpProxyAddress string
	tcpProxyAddress  string
	watchNamespace   string
	userEmail        string
	userPass         string
}

func filterPortManagerConfig(cfg portManagerConfig) portManagerConfig {
	if cfg.image == "" {
		cfg.image = util.GetPortManagerImage()
	}
	if cfg.proxyImage == "" {
		cfg.proxyImage = util.GetProxyImage()
	}
	return cfg
}

func newPortManagerMicroservice(cfg portManagerConfig) *microservice {
	cfg = filterPortManagerConfig(cfg)
	return &microservice{
		mustRecreateOnRollout: true,
		name:                  "port-manager",
		labels: map[string]string{
			"name": "port-manager",
		},
		replicas: 1,
		containers: []container{
			{
				name:            "port-manager",
				image:           cfg.image,
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
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
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
						Value: cfg.watchNamespace,
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
						Value: cfg.userEmail,
					},
					{
						Name:  "IOFOG_USER_PASS",
						Value: cfg.userPass,
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
		ports: []int{
			router.MessagePort,
			router.HTTPPort,
			router.InteriorPort,
			router.EdgePort,
		},
		replicas:      1,
		serviceType:   cfg.serviceType,
		trafficPolicy: getTrafficPolicy(cfg.serviceType),
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
		},
		volumes: []v1.Volume{

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
				env: []v1.EnvVar{
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
						Name:      routerName + "-internal",
						MountPath: cfg.volumeMountPath + "/router-internal",
					},
					{
						Name:      routerName + "-amqps",
						MountPath: cfg.volumeMountPath + "/" + routerName + "-amqps",
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
	return ""
}
