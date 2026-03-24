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

package controllers

import (
	"errors"
	"strconv"
	"strings"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/nats"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
	"github.com/eclipse-iofog/iofog-operator/v3/internal/util"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	routerName                                     = "router"
	controllerName                                 = "controller"
	controllerCredentialsSecretName                = "controller-credentials"
	emailSecretKey                                 = "email"
	passwordSecretKey                              = "password"
	controlllerAuthCredentialsSecretName           = "controller-auth-credentials" //nolint:gosec
	controlllerAuthUrlSecretKey                    = "auth-url"
	controlllerAuthRealmSecretKey                  = "auth-realm"
	controlllerAuthRealmKeySecretKey               = "auth-realm-key"
	controlllerAuthSSLSecretKey                    = "auth-ssl-req"
	controlllerAuthControllerClientSecretKey       = "auth-controller-client"
	controlllerAuthControllerClientSecretSecretKey = "auth-controller-client-secret"
	controlllerAuthViewerClientSecretKey           = "auth-viewer-client"
	controllerDBCredentialsSecretName              = "controller-db-credentials" //nolint:gosec
	controllerVaultCredentialsSecretName           = "controller-vault-credentials"
	controllerDBUserSecretKey                      = "username"
	controllerDBDBNameSecretKey                    = "dbname"
	controllerDBPasswordSecretKey                  = "password"
	controllerDBHostSecretKey                      = "host"
	controllerDBPortSecretKey                      = "port"
	controllerDBSSLSecretKey                       = "ssl"
	controllerDBCACertSecretKey                    = "ca"
)

type service struct {
	name               string
	loadBalancerAddr   string
	trafficPolicy      string
	serviceType        string
	serviceAnnotations map[string]string
	ports              []corev1.ServicePort
	// headless when true sets ClusterIP: None (for StatefulSet headless service)
	headless bool
}

type microservice struct {
	name                  string
	services              []service
	imagePullSecret       string
	replicas              int32
	containers            []container
	initContainers        []container
	labels                map[string]string
	annotations           map[string]string
	secrets               []corev1.Secret
	volumes               []corev1.Volume
	securityContext       *corev1.PodSecurityContext
	rbacRules             []rbacv1.PolicyRule
	mustRecreateOnRollout bool
	availableDelay        int32
	// isStatefulSet when true installs as StatefulSet instead of Deployment (e.g. NATS)
	isStatefulSet          bool
	statefulSetServiceName string // headless service name for StatefulSet (e.g. nats-headless)
	volumeClaimTemplates   []corev1.PersistentVolumeClaim
	// podTemplateAnnotations applied to StatefulSet pod template (e.g. kubectl.kubernetes.io/restartedAt for rollout)
	podTemplateAnnotations map[string]string
}

type container struct {
	name            string
	image           string
	imagePullPolicy string
	args            []string
	readinessProbe  *corev1.Probe
	livenessProbe   *corev1.Probe
	startupProbe    *corev1.Probe
	env             []corev1.EnvVar
	command         []string
	ports           []corev1.ContainerPort
	resources       corev1.ResourceRequirements
	volumeMounts    []corev1.VolumeMount
}

type controllerMicroserviceConfig struct {
	controllerName        string
	replicas              int32
	image                 string
	imagePullSecret       string
	serviceType           string
	serviceAnnotations    map[string]string
	externalTrafficPolicy string
	https                 *bool
	scheme                string
	secretName            string
	loadBalancerAddr      string
	auth                  *cpv3.Auth
	db                    *cpv3.Database
	events                *cpv3.Events
	routerImage           string
	natsImage             string
	natsEnabled           bool
	ecn                   string
	pidBaseDir            string
	ecnViewerPort         int
	ecnViewerURL          string
	logLevel              string
	vault                 *cpv3.Vault
}

func buildControllerSecrets(namespace string, cfg *controllerMicroserviceConfig) []corev1.Secret {
	secrets := []corev1.Secret{
		{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      controllerDBCredentialsSecretName,
			},
			StringData: map[string]string{
				controllerDBDBNameSecretKey:   cfg.db.DatabaseName,
				controllerDBHostSecretKey:     cfg.db.Host,
				controllerDBPortSecretKey:     strconv.Itoa(cfg.db.Port),
				controllerDBUserSecretKey:     cfg.db.User,
				controllerDBPasswordSecretKey: cfg.db.Password,
				controllerDBSSLSecretKey:      getSSLValue(cfg.db.SSL),
				controllerDBCACertSecretKey:   getCAValue(cfg.db.CA),
			},
		},
		{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      controlllerAuthCredentialsSecretName,
			},
			StringData: map[string]string{
				controlllerAuthUrlSecretKey:                    cfg.auth.URL,
				controlllerAuthRealmSecretKey:                  cfg.auth.Realm,
				controlllerAuthRealmKeySecretKey:               cfg.auth.RealmKey,
				controlllerAuthSSLSecretKey:                    cfg.auth.SSL,
				controlllerAuthControllerClientSecretKey:       cfg.auth.ControllerClient,
				controlllerAuthControllerClientSecretSecretKey: cfg.auth.ControllerSecret,
				controlllerAuthViewerClientSecretKey:           cfg.auth.ViewerClient,
			},
		},
	}
	if cfg.vault != nil {
		if vaultSec := buildVaultCredentialsSecret(namespace, cfg.vault); vaultSec != nil {
			secrets = append(secrets, *vaultSec)
		}
	}
	return secrets
}

// buildVaultCredentialsSecret returns a Secret containing provider-specific vault config for the controller. Keys match what we use in SecretKeyRef (address, token, mount for hashicorp; etc.).
func buildVaultCredentialsSecret(namespace string, v *cpv3.Vault) *corev1.Secret {
	if v == nil {
		return nil
	}
	data := make(map[string]string)
	switch {
	case v.Hashicorp != nil:
		data["address"] = v.Hashicorp.Address
		data["token"] = v.Hashicorp.Token
		data["mount"] = v.Hashicorp.Mount
	case v.Aws != nil:
		data["region"] = v.Aws.Region
		data["accessKeyId"] = v.Aws.AccessKeyId
		data["accessKey"] = v.Aws.AccessKey
	case v.Azure != nil:
		data["url"] = v.Azure.URL
		data["tenantId"] = v.Azure.TenantId
		data["clientId"] = v.Azure.ClientId
		data["clientSecret"] = v.Azure.ClientSecret
	case v.Google != nil:
		data["projectId"] = v.Google.ProjectId
		data["credentials"] = v.Google.Credentials
	default:
		return nil
	}
	return &corev1.Secret{
		Type: corev1.SecretTypeOpaque,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      controllerVaultCredentialsSecretName,
		},
		StringData: data,
	}
}

func filterControllerConfig(cfg *controllerMicroserviceConfig) {
	if cfg.replicas == 0 {
		cfg.replicas = 1
	}

	if cfg.image == "" {
		cfg.image = util.GetControllerImage()
	}

	if cfg.routerImage == "" {
		cfg.routerImage = util.GetRouterImage()
	}

	if cfg.natsImage == "" {
		cfg.natsImage = util.GetNatsImage()
	}

	if cfg.serviceType == "" {
		cfg.serviceType = string(corev1.ServiceTypeLoadBalancer)
	}

	if cfg.ecnViewerPort == 0 {
		cfg.ecnViewerPort = 8008
	}

	if cfg.pidBaseDir == "" {
		cfg.pidBaseDir = "/home/runner"
	}

	if cfg.https == nil || *cfg.https == false {
		cfg.scheme = "http"
	} else {
		cfg.scheme = "https"
	}

	if cfg.logLevel == "" {
		cfg.logLevel = "info"
	}

}

func getControllerPort(msvc *microservice) (int, error) {
	if len(msvc.services) == 0 || len(msvc.services[0].ports) == 0 {
		return 0, errors.New("controller microservice does not have requisite ports")
	}

	return int(msvc.services[0].ports[0].Port), nil
}

func newControllerMicroservice(namespace string, cfg *controllerMicroserviceConfig) *microservice {
	filterControllerConfig(cfg)

	msvc := &microservice{
		availableDelay: 5,
		name:           "controller",
		labels: map[string]string{
			"datasance.com/component": "controller",
		},
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"secrets"},
			},
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"apps"},
				Resources: []string{"statefulsets"},
			},
			{
				Verbs:         []string{"update", "patch"},
				APIGroups:     []string{"apps"},
				Resources:     []string{"statefulsets"},
				ResourceNames: []string{"nats"},
			},
		},
		imagePullSecret: cfg.imagePullSecret,
		replicas:        cfg.replicas,
		services: []service{
			{
				name:               "controller",
				serviceType:        cfg.serviceType,
				serviceAnnotations: cfg.serviceAnnotations,
				trafficPolicy:      getTrafficPolicy(cfg.serviceType, cfg.externalTrafficPolicy),
				loadBalancerAddr:   cfg.loadBalancerAddr,
				ports: []corev1.ServicePort{
					{
						Name:       "controller-api",
						Port:       51121,
						TargetPort: intstr.FromInt(51121),
						Protocol:   corev1.Protocol("TCP"),
					},
					{
						Name:       "ecn-viewer",
						Port:       80,
						TargetPort: intstr.FromInt(cfg.ecnViewerPort),
						Protocol:   corev1.Protocol("TCP"),
					},
				},
			},
		},
		secrets: buildControllerSecrets(namespace, cfg),
		volumes: []corev1.Volume{},
		securityContext: &corev1.PodSecurityContext{
			RunAsUser:  ptr.To[int64](10000), // UID
			RunAsGroup: ptr.To[int64](10000), // GID
			FSGroup:    ptr.To[int64](10000), // FSGroup
		},
		containers: []container{
			{
				name:            "controller",
				image:           cfg.image,
				imagePullPolicy: "Always",
				readinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/api/v3/status",
							Port:   intstr.FromInt(51121), //nolint:gomnd
							Scheme: corev1.URIScheme(strings.ToUpper(cfg.scheme)),
						},
					},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      10,
					PeriodSeconds:       5,
					FailureThreshold:    2,
				},
				volumeMounts: []corev1.VolumeMount{},
				env: []corev1.EnvVar{
					{
						Name: "KC_URL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthUrlSecretKey,
							},
						},
					},
					{
						Name: "KC_REALM",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthRealmSecretKey,
							},
						},
					},
					{
						Name: "KC_REALM_KEY",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthRealmKeySecretKey,
							},
						},
					},
					{
						Name: "KC_SSL_REQ",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthSSLSecretKey,
							},
						},
					},
					{
						Name: "KC_CLIENT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthControllerClientSecretKey,
							},
						},
					},
					{
						Name: "KC_CLIENT_SECRET",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthControllerClientSecretSecretKey,
							},
						},
					},
					{
						Name: "KC_VIEWER_CLIENT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controlllerAuthCredentialsSecretName,
								},
								Key: controlllerAuthViewerClientSecretKey,
							},
						},
					},
					{
						Name:  "DB_PROVIDER",
						Value: cfg.db.Provider,
					},
					{
						Name: "DB_NAME",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBDBNameSecretKey,
							},
						},
					},
					{
						Name: "DB_USERNAME",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBUserSecretKey,
							},
						},
					},
					{
						Name: "DB_PASSWORD",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBPasswordSecretKey,
							},
						},
					},
					{
						Name: "DB_HOST",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBHostSecretKey,
							},
						},
					},
					{
						Name: "DB_PORT",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBPortSecretKey,
							},
						},
					},
					{
						Name: "DB_USE_SSL",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBSSLSecretKey,
							},
						},
					},
					{
						Name: "DB_SSL_CA",
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: controllerDBCredentialsSecretName,
								},
								Key: controllerDBCACertSecretKey,
							},
						},
					},
					{
						Name:  "CONTROL_PLANE",
						Value: "Kubernetes",
					},
					{
						Name:  "CONTROLLER_NAME",
						Value: cfg.controllerName,
					},
					{
						Name:  "CONTROLLER_NAMESPACE",
						Value: namespace,
					},
					{
						Name:  "ROUTER_IMAGE_1",
						Value: cfg.routerImage,
					},
					{
						Name:  "ROUTER_IMAGE_2",
						Value: cfg.routerImage,
					},
					{
						Name:  "NATS_ENABLED",
						Value: strconv.FormatBool(cfg.natsEnabled),
					},
					{
						Name:  "NATS_IMAGE_1",
						Value: cfg.natsImage,
					},
					{
						Name:  "NATS_IMAGE_2",
						Value: cfg.natsImage,
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
					{
						Name:  "VIEWER_URL",
						Value: cfg.ecnViewerURL,
					},
					{
						Name:  "LOG_LEVEL",
						Value: cfg.logLevel,
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
		msvc.volumes = append(msvc.volumes, corev1.Volume{
			Name: "controller-sqlite",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "controller-sqlite",
					ReadOnly:  false,
				},
			},
		})

		msvc.containers[0].volumeMounts = append(msvc.containers[0].volumeMounts, corev1.VolumeMount{
			Name:      "controller-sqlite",
			MountPath: "/home/runner/.npm-global/lib/node_modules/@datasance/iofogcontroller/src/data/sqlite_files/",
			// SubPath:   "prod_database.sqlite",
		})
	}

	// Add TLS secret details if type is https and secretname is provided
	if cfg.https != nil && *cfg.https == true {
		msvc.volumes = append(msvc.volumes, corev1.Volume{
			Name: "controller-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: cfg.secretName,
				},
			},
		})

		msvc.containers[0].volumeMounts = append(msvc.containers[0].volumeMounts, corev1.VolumeMount{
			Name:      "controller-cert",
			MountPath: "/etc/pot/controller-cert/",
		})

		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "SERVER_DEV_MODE",
			Value: "false",
		})

		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "SSL_PATH_CERT",
			Value: "/etc/pot/controller-cert/tls.crt",
		})

		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "SSL_PATH_KEY",
			Value: "/etc/pot/controller-cert/tls.key",
		})

		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "SSL_PATH_INTERMEDIATE_CERT",
			Value: "/etc/pot/controller-cert/ca.crt",
		})

	}

	// Add Events environment variables only if events were explicitly configured
	if cfg.events != nil && cfg.events.AuditEnabled != nil {
		// Always set EVENT_AUDIT_ENABLED (true or false)
		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "EVENT_AUDIT_ENABLED",
			Value: strconv.FormatBool(*cfg.events.AuditEnabled),
		})

		// Set optional fields only if audit is enabled
		if *cfg.events.AuditEnabled {
			if cfg.events.RetentionDays != 0 {
				msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
					Name:  "EVENT_RETENTION_DAYS",
					Value: strconv.Itoa(cfg.events.RetentionDays),
				})
			}
			if cfg.events.CleanupInterval != 0 {
				msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
					Name:  "EVENT_CLEANUP_INTERVAL",
					Value: strconv.Itoa(cfg.events.CleanupInterval),
				})
			}
		}

		// Set EVENT_CAPTURE_IP_ADDRESS if explicitly configured
		if cfg.events.CaptureIpAddress != nil {
			msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
				Name:  "EVENT_CAPTURE_IP_ADDRESS",
				Value: strconv.FormatBool(*cfg.events.CaptureIpAddress),
			})
		}
	}

	// Vault: optional. When configured, set VAULT_* env from spec and from operator-created Secret (provider-specific).
	if cfg.vault != nil {
		enabled := true
		if cfg.vault.Enabled != nil {
			enabled = *cfg.vault.Enabled
		}
		msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
			Name:  "VAULT_ENABLED",
			Value: strconv.FormatBool(enabled),
		})
		if cfg.vault.Provider != "" {
			msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
				Name:  "VAULT_PROVIDER",
				Value: cfg.vault.Provider,
			})
		}
		if cfg.vault.BasePath != "" {
			basePath := strings.ReplaceAll(cfg.vault.BasePath, "$namespace", namespace)
			msvc.containers[0].env = append(msvc.containers[0].env, corev1.EnvVar{
				Name:  "VAULT_BASE_PATH",
				Value: basePath,
			})
		}
		// Provider-specific env vars from the operator-created Secret (keys: address, token, mount; region, accessKeyId, accessKey; etc.)
		switch {
		case cfg.vault.Hashicorp != nil:
			msvc.containers[0].env = append(msvc.containers[0].env,
				corev1.EnvVar{Name: "VAULT_HASHICORP_ADDRESS", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "address"}}},
				corev1.EnvVar{Name: "VAULT_HASHICORP_TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "token"}}},
				corev1.EnvVar{Name: "VAULT_HASHICORP_MOUNT", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "mount"}}},
			)
		case cfg.vault.Aws != nil:
			msvc.containers[0].env = append(msvc.containers[0].env,
				corev1.EnvVar{Name: "VAULT_AWS_REGION", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "region"}}},
				corev1.EnvVar{Name: "VAULT_AWS_ACCESS_KEY_ID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "accessKeyId"}}},
				corev1.EnvVar{Name: "VAULT_AWS_ACCESS_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "accessKey"}}},
			)
		case cfg.vault.Azure != nil:
			msvc.containers[0].env = append(msvc.containers[0].env,
				corev1.EnvVar{Name: "VAULT_AZURE_URL", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "url"}}},
				corev1.EnvVar{Name: "VAULT_AZURE_TENANT_ID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "tenantId"}}},
				corev1.EnvVar{Name: "VAULT_AZURE_CLIENT_ID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "clientId"}}},
				corev1.EnvVar{Name: "VAULT_AZURE_CLIENT_SECRET", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "clientSecret"}}},
			)
		case cfg.vault.Google != nil:
			msvc.containers[0].env = append(msvc.containers[0].env,
				corev1.EnvVar{Name: "VAULT_GOOGLE_PROJECT_ID", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "projectId"}}},
				corev1.EnvVar{Name: "VAULT_GOOGLE_CREDENTIALS", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: controllerVaultCredentialsSecretName}, Key: "credentials"}}},
			)
		}
	}

	return msvc
}

type routerMicroserviceConfig struct {
	image                 string
	imagePullSecret       string
	serviceType           string
	serviceAnnotations    map[string]string
	externalTrafficPolicy string
	siteCA                string
	localCA               string
	siteSecret            string
	localSecret           string
	ha                    bool
}

func filterRouterConfig(cfg routerMicroserviceConfig) routerMicroserviceConfig {
	if cfg.image == "" {
		cfg.image = util.GetRouterImage()
	}

	if cfg.serviceType == "" {
		cfg.serviceType = string(corev1.ServiceTypeLoadBalancer)
	}

	if cfg.siteSecret == "" {
		cfg.siteSecret = "router-site-server"
	}

	if cfg.localSecret == "" {
		cfg.localSecret = "router-local-server"
	}

	if cfg.siteCA == "" {
		cfg.siteCA = "router-site-ca"
	}

	if cfg.localCA == "" {
		cfg.localCA = "default-router-local-ca"
	}

	return cfg
}

func newRouterMicroservice(cfg routerMicroserviceConfig) *microservice {
	cfg = filterRouterConfig(cfg)

	return &microservice{
		name: routerName,
		labels: map[string]string{
			"datasance.com/component": routerName,
			"application":             "interior-router",
			"skupper.io/component":    "router",
			"skupper.io/type":         "site",
		},
		annotations: map[string]string{
			"prometheus.io/port":   "9090",
			"prometheus.io/scrape": "true",
		},
		services: []service{
			{
				name:               "router",
				serviceType:        cfg.serviceType,
				serviceAnnotations: cfg.serviceAnnotations,
				trafficPolicy:      getTrafficPolicy(cfg.serviceType, cfg.externalTrafficPolicy),
				ports: []corev1.ServicePort{
					{
						Name:       "router-message",
						Port:       router.MessagePort,
						TargetPort: intstr.FromInt(router.MessagePort),
						Protocol:   corev1.Protocol("TCP"),
					},
					{
						Name:       "router-interior",
						Port:       router.InteriorPort,
						TargetPort: intstr.FromInt(router.InteriorPort),
						Protocol:   corev1.Protocol("TCP"),
					},
					{
						Name:       "router-edge",
						Port:       router.EdgePort,
						TargetPort: intstr.FromInt(router.EdgePort),
						Protocol:   corev1.Protocol("TCP"),
					},
				},
			},
			// {
			// 	name:          "router-internal",
			// 	serviceType:   "ClusterIP",
			// 	trafficPolicy: getTrafficPolicy("ClusterIP"),
			// 	ports: []corev1.ServicePort{
			// 		{
			// 			Name:       "router-internal",
			// 			Port:       5672,
			// 			TargetPort: intstr.FromInt(5672),
			// 			Protocol:   corev1.Protocol("TCP"),
			// 		},
			// 	},
			// },
		},
		imagePullSecret: cfg.imagePullSecret,
		replicas:        1,
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"secrets"},
			},
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
			},
			{
				Verbs:     []string{"get"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
		},
		volumes: []corev1.Volume{
			{
				Name: "iofog-router-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "iofog-router"},
						Items: []corev1.KeyToPath{
							{Key: "skrouterd.json", Path: "skrouterd.json"},
						},
					},
				},
			},
			{
				Name: "router-site-server",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "router-site-server",
					},
				},
			},
			{
				Name: "router-local-server",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "router-local-server",
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
					"/home/skrouterd/bin/router",
				},
				ports: []corev1.ContainerPort{
					{
						Name:          "amqps",
						ContainerPort: 5671,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "http",
						ContainerPort: 9090,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "inter-router",
						ContainerPort: 55671,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "edge",
						ContainerPort: 45671,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				readinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port:   intstr.FromInt(9090),
							Path:   "/healthz",
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 1,
					TimeoutSeconds:      1,
					PeriodSeconds:       10,
					FailureThreshold:    3,
					SuccessThreshold:    1,
				},
				livenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port:   intstr.FromInt(9090),
							Path:   "/healthz",
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 60,
					TimeoutSeconds:      1,
					PeriodSeconds:       10,
					FailureThreshold:    3,
					SuccessThreshold:    1,
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
						Value: "/tmp/skrouterd.json",
					},
					{
						Name:  "QDROUTERD_CONF_TYPE",
						Value: "json",
					},
					{
						Name:  "SSL_PROFILE_PATH",
						Value: "/etc/skupper-router-certs",
					},
					{
						Name:  "SKUPPER_SITE_ID",
						Value: "default-router",
					},
					{
						Name:  "SKUPPER_PLATFORM",
						Value: "kubernetes",
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
						Name:      "iofog-router-config",
						MountPath: "/tmp",
					},
					{
						Name:      "router-site-server",
						MountPath: "/etc/skupper-router-certs/router-site-server",
					},
					{
						Name:      "router-local-server",
						MountPath: "/etc/skupper-router-certs/router-local-server",
					},
				},
			},
		},
	}
}

// newRouterMicroservices creates router microservices based on HA configuration
func newRouterMicroservices(cfg routerMicroserviceConfig) []*microservice {
	cfg = filterRouterConfig(cfg)

	var microservices []*microservice

	// Always create the primary router
	primaryRouter := newRouterMicroservice(cfg)
	microservices = append(microservices, primaryRouter)

	// Create secondary router if HA is enabled
	if cfg.ha {
		secondaryRouter := newRouterMicroserviceWithName(cfg, "router-2")
		microservices = append(microservices, secondaryRouter)
	}

	return microservices
}

type natsMicroserviceConfig struct {
	image                       string
	imagePullSecret             string
	replicas                    int32
	storageSize                 string
	storageClassName            string
	jetStreamKeySecret          string
	serviceType                 string
	serviceAnnotations          map[string]string
	externalTrafficPolicy       string
	serverServiceType           string
	serverServiceAnnotations    map[string]string
	serverExternalTrafficPolicy string
}

func newNatsMicroservice(cfg natsMicroserviceConfig) *microservice {
	// Headless: all ports (StatefulSet pod discovery: nats-0.nats-headless, etc.).
	headlessPorts := []corev1.ServicePort{
		{Name: "cluster", Port: int32(nats.DefaultClusterPort), TargetPort: intstr.FromInt(nats.DefaultClusterPort)},
		{Name: "leaf", Port: int32(nats.DefaultLeafPort), TargetPort: intstr.FromInt(nats.DefaultLeafPort)},
		{Name: "mqtt", Port: int32(nats.DefaultMqttPort), TargetPort: intstr.FromInt(nats.DefaultMqttPort)},
		{Name: "client", Port: int32(nats.DefaultServerPort), TargetPort: intstr.FromInt(nats.DefaultServerPort)},
		{Name: "monitor", Port: int32(nats.DefaultHttpPort), TargetPort: intstr.FromInt(nats.DefaultHttpPort)},
	}
	// Client-facing: cluster, leaf, mqtt (LB/ingress and spec.services.nats apply here).
	clientPorts := []corev1.ServicePort{
		{Name: "cluster", Port: int32(nats.DefaultClusterPort), TargetPort: intstr.FromInt(nats.DefaultClusterPort)},
		{Name: "leaf", Port: int32(nats.DefaultLeafPort), TargetPort: intstr.FromInt(nats.DefaultLeafPort)},
		{Name: "mqtt", Port: int32(nats.DefaultMqttPort), TargetPort: intstr.FromInt(nats.DefaultMqttPort)},
	}
	// nats-server: client and monitor only (ClusterIP).
	serverPorts := []corev1.ServicePort{
		{Name: "client", Port: int32(nats.DefaultServerPort), TargetPort: intstr.FromInt(nats.DefaultServerPort)},
		{Name: "monitor", Port: int32(nats.DefaultHttpPort), TargetPort: intstr.FromInt(nats.DefaultHttpPort)},
	}

	storageQuantity := resource.MustParse(cfg.storageSize)
	if storageQuantity.IsZero() {
		storageQuantity = resource.MustParse(nats.DefaultStorageSizePVC)
	}
	pvcSpec := corev1.PersistentVolumeClaimSpec{
		AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
		Resources:   corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: storageQuantity}},
	}
	if cfg.storageClassName != "" {
		pvcSpec.StorageClassName = ptr.To(cfg.storageClassName)
	}

	return &microservice{
		name:                   "nats",
		isStatefulSet:          true,
		statefulSetServiceName: nats.HeadlessServiceName,
		volumeClaimTemplates:   []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "js-data"}, Spec: pvcSpec}},
		imagePullSecret:        cfg.imagePullSecret,
		replicas:               cfg.replicas,
		labels:                 map[string]string{"datasance.com/component": "nats"},
		services: []service{
			{name: nats.HeadlessServiceName, serviceType: "ClusterIP", headless: true, ports: headlessPorts},
			{name: nats.ClientServiceName, serviceType: cfg.serviceType, serviceAnnotations: cfg.serviceAnnotations, trafficPolicy: getTrafficPolicy(cfg.serviceType, cfg.externalTrafficPolicy), ports: clientPorts},
			{name: nats.ServerServiceName, serviceType: cfg.serverServiceType, serviceAnnotations: cfg.serverServiceAnnotations, trafficPolicy: getTrafficPolicy(cfg.serverServiceType, cfg.serverExternalTrafficPolicy), headless: false, ports: serverPorts},
		},
		volumes: []corev1.Volume{
			{Name: "config", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: nats.ConfigMapName}}}},
			{Name: "jwt", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: nats.JWTBundleCMName}}}},
			{Name: "nats-site-server", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nats.NatsSiteServerSecret}}},
			{Name: "nats-mqtt-server", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nats.NatsMqttServerSecret}}},
			{Name: "jetstream-key", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: cfg.jetStreamKeySecret}}},
			{Name: "sys-user-creds", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: nats.HubSystemUserCredsSecret, Items: []corev1.KeyToPath{{Key: nats.HubSystemUserCredsDataKey, Path: "admin-hub.creds"}}}}},
		},
		containers: []container{
			{
				name:            "nats",
				image:           cfg.image,
				imagePullPolicy: "Always",
				ports: []corev1.ContainerPort{
					{Name: "client", ContainerPort: nats.DefaultServerPort},
					{Name: "cluster", ContainerPort: nats.DefaultClusterPort},
					{Name: "leaf", ContainerPort: nats.DefaultLeafPort},
					{Name: "mqtt", ContainerPort: nats.DefaultMqttPort},
					{Name: "monitor", ContainerPort: nats.DefaultHttpPort},
				},
				env: []corev1.EnvVar{
					{Name: "NATS_CONF", Value: "/etc/nats/config/server.conf"},
					{Name: "NATS_SERVER_MODE", Value: "server"},
					{Name: "NATS_JWT_DIR", Value: "/home/runner/nats/jwt"},
					{Name: "NATS_JWT_MOUNT_DIR", Value: "/tmp/nats/jwt"},
					{Name: "NATS_CREDS_DIR", Value: "/etc/nats/creds"},
					{Name: "NATS_SYS_USER_CRED_PATH", Value: "/etc/nats/creds/admin-hub.creds"},
					{Name: "NATS_SSL_DIR", Value: "/etc/nats/certs"},
					{Name: "NATS_CERT_NAME", Value: "nats-site-server"},
					{Name: "NATS_MQTT_CERT_NAME", Value: "nats-mqtt-server"},
					{Name: "NATS_SERVER_PORT", Value: "4222"},
					{Name: "NATS_CLUSTER_PORT", Value: "6222"},
					{Name: "NATS_LEAF_PORT", Value: "7422"},
					{Name: "NATS_MQTT_PORT", Value: "8883"},
					{Name: "NATS_MONITOR_PORT", Value: "8222"},
					{Name: "NATS_JETSTREAM_STORE_DIR", Value: "/home/runner/data"},
					{Name: "NATS_HTTP_PORT", Value: "8222"},
					{Name: "JETSTREAM_KEY", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: cfg.jetStreamKeySecret}, Key: "jsk"}}},
					{Name: "JETSTREAM_PREV_KEY", Value: ""},
					{Name: "SELFNAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
				},
				volumeMounts: []corev1.VolumeMount{
					{Name: "config", MountPath: "/etc/nats/config"},
					{Name: "jwt", MountPath: "/tmp/nats/jwt"},
					{Name: "nats-site-server", MountPath: "/etc/nats/certs/nats-site-server"},
					{Name: "nats-mqtt-server", MountPath: "/etc/nats/certs/nats-mqtt-server"},
					{Name: "jetstream-key", MountPath: "/etc/nats/jetstream", ReadOnly: true},
					{Name: "sys-user-creds", MountPath: "/etc/nats/creds", ReadOnly: true},
					{Name: "js-data", MountPath: "/home/runner/data"},
				},
				readinessProbe: &corev1.Probe{
					ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz?js-enabled-only=true", Port: intstr.FromInt32(nats.DefaultHttpPort)}},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      5,
					PeriodSeconds:       10,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				livenessProbe: &corev1.Probe{
					ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz?js-enabled-only=true", Port: intstr.FromInt32(nats.DefaultHttpPort)}},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      5,
					PeriodSeconds:       30,
					FailureThreshold:    3,
					SuccessThreshold:    1,
				},
				// startupProbe: &corev1.Probe{
				// 	ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt32(nats.DefaultHttpPort)}},
				// 	InitialDelaySeconds: 10,
				// 	TimeoutSeconds:      5,
				// 	PeriodSeconds:       10,
				// 	SuccessThreshold:    1,
				// 	FailureThreshold:    90,
				// },
			},
		},
		rbacRules: []rbacv1.PolicyRule{
			{Verbs: []string{"get", "list", "watch"}, APIGroups: []string{""}, Resources: []string{"configmaps", "secrets"}},
		},
	}
}

// newRouterMicroserviceWithName creates a router microservice with a custom name
func newRouterMicroserviceWithName(cfg routerMicroserviceConfig, name string) *microservice {
	cfg = filterRouterConfig(cfg)

	return &microservice{
		name: name,
		labels: map[string]string{
			"datasance.com/component": routerName,
			"application":             "interior-router",
			"skupper.io/component":    "router",
			"skupper.io/type":         "site",
		},
		annotations: map[string]string{
			"prometheus.io/port":   "9090",
			"prometheus.io/scrape": "true",
		},
		services:        []service{}, // Secondary router doesn't create its own service
		imagePullSecret: cfg.imagePullSecret,
		replicas:        1,
		rbacRules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{""},
				Resources: []string{"pods", "services", "endpoints"},
			},
			{
				Verbs:     []string{"get", "list", "watch"},
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
			},
		},
		volumes: []corev1.Volume{
			{
				Name: "iofog-router-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "iofog-router"},
						Items: []corev1.KeyToPath{
							{Key: "skrouterd.json", Path: "skrouterd.json"},
						},
					},
				},
			},
			{
				Name: "router-site-server",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "router-site-server",
					},
				},
			},
			{
				Name: "router-local-server",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "router-local-server",
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
					"/home/skrouterd/bin/launch.sh",
				},
				ports: []corev1.ContainerPort{
					{
						Name:          "amqps",
						ContainerPort: 5671,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "http",
						ContainerPort: 9090,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "inter-router",
						ContainerPort: 55671,
						Protocol:      corev1.ProtocolTCP,
					},
					{
						Name:          "edge",
						ContainerPort: 45671,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				readinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port:   intstr.FromInt(9090),
							Path:   "/healthz",
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 1,
					TimeoutSeconds:      1,
					PeriodSeconds:       10,
					FailureThreshold:    3,
					SuccessThreshold:    1,
				},
				livenessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{
							Port:   intstr.FromInt(9090),
							Path:   "/healthz",
							Scheme: corev1.URISchemeHTTP,
						},
					},
					InitialDelaySeconds: 60,
					TimeoutSeconds:      1,
					PeriodSeconds:       10,
					FailureThreshold:    3,
					SuccessThreshold:    1,
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
						Value: "/tmp/skrouterd.json",
					},
					{
						Name:  "QDROUTERD_CONF_TYPE",
						Value: "json",
					},
					{
						Name:  "SSL_PROFILE_PATH",
						Value: "/etc/skupper-router-certs",
					},
					{
						Name:  "SKUPPER_SITE_ID",
						Value: "default-router",
					},
					{
						Name:  "SKUPPER_SITE_NAME",
						Value: "default-router",
					},
					{
						Name:  "SKUPPER_PLATFORM",
						Value: "kubernetes",
					},
				},
				volumeMounts: []corev1.VolumeMount{
					{
						Name:      "iofog-router-config",
						MountPath: "/tmp",
					},
					{
						Name:      "router-site-server",
						MountPath: "/etc/skupper-router-certs/router-site-server",
					},
					{
						Name:      "router-local-server",
						MountPath: "/etc/skupper-router-certs/router-local-server",
					},
				},
			},
		},
	}
}

func getSSLValue(ssl *bool) string {
	if ssl == nil {
		return "false"
	}
	return strconv.FormatBool(*ssl)
}

func getCAValue(ca *string) string {
	if ca == nil {
		return ""
	}
	return *ca
}

// getTrafficPolicy returns the externalTrafficPolicy for a service. Only LoadBalancer and NodePort
// may have this field set; for ClusterIP it must be empty. When override is "Local" or "Cluster" it
// is used; when omitted, LoadBalancer defaults to Local and NodePort to Cluster.
func getTrafficPolicy(serviceType string, override string) string {
	if strings.EqualFold(serviceType, string(corev1.ServiceTypeClusterIP)) {
		return ""
	}
	if strings.EqualFold(override, string(corev1.ServiceExternalTrafficPolicyTypeLocal)) {
		return string(corev1.ServiceExternalTrafficPolicyTypeLocal)
	}
	if strings.EqualFold(override, string(corev1.ServiceExternalTrafficPolicyTypeCluster)) {
		return string(corev1.ServiceExternalTrafficPolicyTypeCluster)
	}
	if strings.EqualFold(serviceType, string(corev1.ServiceTypeLoadBalancer)) {
		return string(corev1.ServiceExternalTrafficPolicyTypeLocal)
	}
	if strings.EqualFold(serviceType, string(corev1.ServiceTypeNodePort)) {
		return string(corev1.ServiceExternalTrafficPolicyTypeCluster)
	}
	return ""
}
