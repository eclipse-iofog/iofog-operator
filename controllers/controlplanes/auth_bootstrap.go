package controllers

import (
	"context"
	"fmt"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func effectiveBootstrapPassword(auth *cpv3.Auth, resolved string) string {
	if resolved != "" {
		return resolved
	}
	if auth != nil && auth.Bootstrap != nil {
		return auth.Bootstrap.Password
	}
	return ""
}

func hasBootstrapPassword(auth *cpv3.Auth, resolved string) bool {
	return effectiveBootstrapPassword(auth, resolved) != ""
}

func resolveBootstrapPassword(ctx context.Context, c client.Client, namespace string, auth *cpv3.Auth) (string, error) {
	if auth == nil || auth.Mode != cpv3.AuthModeEmbedded || auth.Bootstrap == nil {
		return "", nil
	}
	bootstrap := auth.Bootstrap
	if bootstrap.PasswordSecretRef != nil {
		return readSecretKeyValue(ctx, c, namespace, bootstrap.PasswordSecretRef)
	}
	return bootstrap.Password, nil
}

func readSecretKeyValue(ctx context.Context, c client.Client, namespace string, ref *corev1.SecretKeySelector) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("passwordSecretRef is required")
	}
	if ref.Name == "" {
		return "", fmt.Errorf("passwordSecretRef.name is required")
	}
	if ref.Key == "" {
		return "", fmt.Errorf("passwordSecretRef.key is required")
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ref.Name}, secret); err != nil {
		return "", fmt.Errorf("get bootstrap password secret %q: %w", ref.Name, err)
	}

	raw, ok := secret.Data[ref.Key]
	if !ok {
		if ref.Optional != nil && *ref.Optional {
			return "", nil
		}
		return "", fmt.Errorf("bootstrap password secret %q missing key %q", ref.Name, ref.Key)
	}
	return string(raw), nil
}
