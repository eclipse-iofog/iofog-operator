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
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// BootstrapFromAPI is the response from GET /api/v3/nats/bootstrap (Controller).
// Controller performs bootstrap and returns this; operator only persists to K8s secrets.
// SysUserCredsBase64 is the hub system user creds file encoded in base64.
type BootstrapFromAPI struct {
	OperatorJwt            string
	OperatorPublicKey      string
	OperatorSeed           string
	SystemAccountJwt       string
	SystemAccountPublicKey string
	SysUserCredsBase64     string
}

// Controller-compatible secret names and data keys (see nats-auth-service.js).
const (
	OperatorSeedSecretName    = "nats-operator-seed"
	SystemAccountSeedSecret   = "nats-system-account-seed"
	HubSystemUserCredsSecret  = "nats-creds-sys-admin-hub"
	SystemAccountName         = "SYS"
	HubSystemUserName         = "admin-hub"
	HubSystemUserCredsDataKey = "admin-hub.creds"
)

// NatsBootstrapSecrets holds the secrets created by NATS bootstrap (for use in ConfigMap and mounts).
type NatsBootstrapSecrets struct {
	OperatorJWT         string
	SystemAccountJWT    string
	SystemAccountPubKey string // for JWT bundle key ${pubKey}.jwt
	CredsContent        string
}

// EnsureNatsBootstrapFromController saves NATS bootstrap data from the Controller API into K8s secrets.
// The Controller handles bootstrap (GET /api/v3/nats/bootstrap); the operator only persists.
// SysUserCredsBase64 in the response is decoded from base64 before storing in the hub creds secret.
func EnsureNatsBootstrapFromController(ctx context.Context, createSecret func(context.Context, *corev1.Secret) error, namespace string, labels map[string]string, api *BootstrapFromAPI) (*NatsBootstrapSecrets, error) {
	if api == nil {
		return nil, fmt.Errorf("bootstrap API response is nil")
	}
	credsRaw, err := base64.StdEncoding.DecodeString(api.SysUserCredsBase64)
	if err != nil {
		return nil, fmt.Errorf("decode sys user creds base64: %w", err)
	}
	opSeed := []byte(api.OperatorSeed)
	opSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: OperatorSeedSecretName, Namespace: namespace, Labels: labels},
		Data:       map[string][]byte{"seed": opSeed},
	}
	if err := createSecret(ctx, opSecret); err != nil {
		return nil, fmt.Errorf("create operator seed secret: %w", err)
	}
	credsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: HubSystemUserCredsSecret, Namespace: namespace, Labels: labels},
		Data:       map[string][]byte{HubSystemUserCredsDataKey: credsRaw},
	}
	if err := createSecret(ctx, credsSecret); err != nil {
		return nil, fmt.Errorf("create hub system user creds secret: %w", err)
	}
	return &NatsBootstrapSecrets{
		OperatorJWT:         api.OperatorJwt,
		SystemAccountJWT:    api.SystemAccountJwt,
		SystemAccountPubKey: api.SystemAccountPublicKey,
		CredsContent:        string(credsRaw),
	}, nil
}

// EnsureJetStreamKeySecret ensures the JetStream encryption key secret exists (nats-jetstream-key-<controlplaneName>).
// Key in data is "jsk", value is 32 random bytes base64-encoded.
func EnsureJetStreamKeySecret(ctx context.Context, getSecret func(context.Context, types.NamespacedName, *corev1.Secret) error, createSecret func(context.Context, *corev1.Secret) error, namespace, controlplaneName string, labels map[string]string) (created bool, err error) {
	name := JetStreamKeySecretName(controlplaneName)
	existing := &corev1.Secret{}
	err = getSecret(ctx, types.NamespacedName{Name: name, Namespace: namespace}, existing)
	if err == nil && len(existing.Data["jsk"]) > 0 {
		return false, nil
	}
	if err != nil && !k8serrors.IsNotFound(err) {
		return false, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return false, fmt.Errorf("jetstream key rand: %w", err)
	}
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: labels},
		Data:       map[string][]byte{"jsk": []byte(base64.StdEncoding.EncodeToString(key))},
	}
	if err := createSecret(ctx, s); err != nil {
		return false, err
	}
	return true, nil
}
