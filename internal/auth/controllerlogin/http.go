package controllerlogin

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// httpClient uses Go's default verifying transport when tlsConfig is nil.
func httpClient(tlsConfig *tls.Config) *http.Client {
	client := &http.Client{Timeout: 10 * time.Second}
	if tlsConfig != nil {
		client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}
	return client
}

// issuerTLSConfig trusts the system roots plus an optional Base64 encoded PEM CA bundle.
// An empty issuerCa returns nil so the caller uses the default verifying client.
func issuerTLSConfig(issuerCa string) (*tls.Config, error) {
	issuerCa = strings.TrimSpace(issuerCa)
	if issuerCa == "" {
		return nil, nil
	}

	raw, err := base64.StdEncoding.DecodeString(issuerCa)
	if err != nil {
		return nil, fmt.Errorf("decode auth.issuerCa: %w", err)
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(raw) {
		return nil, fmt.Errorf("auth.issuerCa does not contain a PEM certificate")
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}, nil
}
