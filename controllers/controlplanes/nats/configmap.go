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
)

const serverConfKey = "server.conf"

// ServerConfKey returns the ConfigMap data key for server.conf (used when reading existing config for route merge).
func ServerConfKey() string {
	return serverConfKey
}

// NewNatsConfigMap creates the iofog-nats-config ConfigMap with server.conf content.
func NewNatsConfigMap(namespace, instanceName string, labels map[string]string, serverConfContent string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			serverConfKey: serverConfContent,
		},
	}
}

// NewJWTBundleConfigMap creates the iofog-nats-jwt-bundle ConfigMap with account JWTs only.
// Keys are ${accountPublicKey}.jwt (e.g. for system account: ACxxxxx.jwt).
// At bootstrap pass a single entry: systemAccountPublicKey -> systemAccountJWT.
func NewJWTBundleConfigMap(namespace string, labels map[string]string, accountJWTs map[string]string) *corev1.ConfigMap {
	data := make(map[string]string, len(accountJWTs))
	for pubKey, jwtContent := range accountJWTs {
		key := pubKey + ".jwt"
		data[key] = jwtContent
	}
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      JWTBundleCMName,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}
