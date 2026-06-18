/*
 *  *******************************************************************************
 *  * Copyright (c) 2023 Datasance Teknoloji A.S.
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
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	util "github.com/datasance/iofog-operator/v3/internal/util/certs"
)

// NATS TLS secret names (aligned with Controller nats-service.js).
const (
	NatsSiteCASecret     = "nats-site-ca"
	NatsSiteServerSecret = "nats-site-server"
	NatsLocalCASecret    = "default-nats-local-ca"
	NatsMqttServerSecret = "nats-mqtt-server"
)

// natsSiteServerHosts returns comma-separated SANs for the NATS site server cert (same pattern as router createRouterSecrets):
// internal: nats-0.<headless>, nats-1.<headless>, ..., *.headless.ns.svc.cluster.local, nats.ns.svc.cluster.local;
// when address is non-empty (LB host or ingress host), also include it so external clients validate.
func natsSiteServerHosts(headlessName, namespace string, replicas int, address string) string {
	hosts := ""
	for i := 0; i < replicas; i++ {
		if i > 0 {
			hosts += ","
		}
		hosts += fmt.Sprintf("nats-%d.%s", i, headlessName)
	}
	hosts += fmt.Sprintf(",%s.%s.svc.cluster.local", ClientServiceName, namespace)
	hosts += fmt.Sprintf(",%s.%s.svc.cluster.local", ServerServiceName, namespace)
	hosts += fmt.Sprintf(",*.%s.%s.svc.cluster.local", headlessName, namespace)
	hosts += fmt.Sprintf(",nats.%s.svc.cluster.local", namespace)
	hosts += fmt.Sprintf(",nats-server.%s.svc.cluster.local", namespace)
	if address != "" {
		hosts += "," + address
	}
	return hosts
}

// EnsureNatsSecrets ensures NATS TLS secrets exist in the namespace. Creates nats-site-ca, nats-site-server,
// default-nats-local-ca, nats-mqtt-server. Uses same pattern as router (util.GenerateSecret).
// headlessName is typically "nats-headless". address is the external host (LB or ingress) for SANs, like createRouterSecrets.
func EnsureNatsSecrets(ctx context.Context, getSecret func(context.Context, types.NamespacedName, *corev1.Secret) error, namespace, instanceName, headlessName string, replicas int, address string, labels map[string]string) ([]corev1.Secret, error) {
	siteCA := &corev1.Secret{}
	localCA := &corev1.Secret{}
	siteServer := &corev1.Secret{}
	mqttServer := &corev1.Secret{}

	err := getSecret(ctx, types.NamespacedName{Name: NatsSiteCASecret, Namespace: namespace}, siteCA)
	siteCAExists := err == nil
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	err = getSecret(ctx, types.NamespacedName{Name: NatsLocalCASecret, Namespace: namespace}, localCA)
	localCAExists := err == nil
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	err = getSecret(ctx, types.NamespacedName{Name: NatsSiteServerSecret, Namespace: namespace}, siteServer)
	siteServerExists := err == nil
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	err = getSecret(ctx, types.NamespacedName{Name: NatsMqttServerSecret, Namespace: namespace}, mqttServer)
	mqttServerExists := err == nil
	if err != nil && !k8serrors.IsNotFound(err) {
		return nil, err
	}

	var out []corev1.Secret

	if !siteCAExists {
		s := util.GenerateSecret(NatsSiteCASecret, NatsSiteCASecret, NatsSiteCASecret, 0, nil)
		s.Namespace = namespace
		s.Labels = labels
		out = append(out, s)
		siteCA = &s
	}

	if !localCAExists {
		s := util.GenerateSecret(NatsLocalCASecret, NatsLocalCASecret, NatsLocalCASecret, 0, nil)
		s.Namespace = namespace
		s.Labels = labels
		out = append(out, s)
		localCA = &s
	}

	if !siteServerExists {
		hosts := natsSiteServerHosts(headlessName, namespace, replicas, address)
		s := util.GenerateSecret(NatsSiteServerSecret, "iofog-nats", hosts, 0, siteCA)
		s.Namespace = namespace
		s.Labels = labels
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations["datasance.com/nats-replicas"] = strconv.Itoa(replicas)
		out = append(out, s)
	}

	if !mqttServerExists {
		// MQTT server cert: same SANs as site server for NATS client connectivity
		hosts := natsSiteServerHosts(headlessName, namespace, replicas, address)
		s := util.GenerateSecret(NatsMqttServerSecret, "iofog-nats-mqtt", hosts, 0, localCA)
		s.Namespace = namespace
		s.Labels = labels
		if s.Annotations == nil {
			s.Annotations = make(map[string]string)
		}
		s.Annotations["datasance.com/nats-replicas"] = strconv.Itoa(replicas)
		out = append(out, s)
	}

	return out, nil
}
