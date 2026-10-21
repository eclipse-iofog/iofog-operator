package controllers

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"strings"

	op "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/k8s/operator"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const controllerTLSCAKey = "ca.crt"

// controllerClientTLSConfig verifies the Controller certificate against the public URL host
// while the operator dials the in-cluster Service. caPEM is appended to the system pool when
// it contains a PEM certificate.
func controllerClientTLSConfig(publicURL string, caPEM []byte) (*tls.Config, error) {
	host, err := publicURLHost(publicURL)
	if err != nil {
		return nil, err
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if len(strings.TrimSpace(string(caPEM))) > 0 {
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("controller TLS secret %s does not contain a PEM certificate", controllerTLSCAKey)
		}
	}
	return &tls.Config{ServerName: host, RootCAs: pool}, nil
}

func publicURLHost(publicURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(publicURL))
	if err != nil {
		return "", fmt.Errorf("parse controller public URL: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("controller.https requires a public URL hostname or IP")
	}
	return host, nil
}

// controllerClientTLS returns nil when Controller HTTPS is off. The dial address stays the
// cluster FQDN or LoadBalancer address; ServerName is the resolved public URL host.
func (r *ControlPlaneReconciler) controllerClientTLS(ctx context.Context) (*tls.Config, op.Reconciliation) {
	if !controllerHTTPSFromSpec(r.cp.Spec) {
		return nil, op.Continue()
	}

	lbIP := r.serviceLoadBalancerAddress(ctx, r.cp.Namespace, controllerName)
	access, needLBIP := resolveControllerAccess(r.cp.Spec, lbIP)
	if needLBIP {
		return nil, r.requeueForPendingLoadBalancer(r.cp.Namespace, controllerName)
	}

	caPEM, err := r.controllerCAPEM(ctx)
	if err != nil {
		return nil, op.ReconcileWithError(err)
	}
	cfg, err := controllerClientTLSConfig(access.PublicURL, caPEM)
	if err != nil {
		return nil, op.ReconcileWithError(err)
	}
	return cfg, op.Continue()
}

func (r *ControlPlaneReconciler) controllerCAPEM(ctx context.Context) ([]byte, error) {
	name := strings.TrimSpace(r.cp.Spec.Controller.SecretName)
	if name == "" {
		return nil, nil
	}

	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: r.cp.Namespace}, secret)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get controller TLS secret %s: %w", name, err)
	}
	if len(secret.Data) == 0 {
		return nil, nil
	}
	return secret.Data[controllerTLSCAKey], nil
}
