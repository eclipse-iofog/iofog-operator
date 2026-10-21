package controllers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestControllerClientTLSConfig_ServerNameDNSAndIP(t *testing.T) {
	system, err := x509.SystemCertPool()
	require.NoError(t, err)
	require.NotNil(t, system)
	systemSubjects := len(system.Subjects())

	caPEM := testControllerCAPEM(t)
	cfg, err := controllerClientTLSConfig("https://controller.example.com", caPEM)
	require.NoError(t, err)
	require.Equal(t, "controller.example.com", cfg.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.Greater(t, len(cfg.RootCAs.Subjects()), systemSubjects)
	require.False(t, cfg.InsecureSkipVerify)

	cfg, err = controllerClientTLSConfig("https://203.0.113.10:51121", nil)
	require.NoError(t, err)
	require.Equal(t, "203.0.113.10", cfg.ServerName)
	require.Equal(t, uint16(tls.VersionTLS12), cfg.MinVersion)
	require.GreaterOrEqual(t, len(cfg.RootCAs.Subjects()), systemSubjects)
	require.False(t, cfg.InsecureSkipVerify)
}

func TestControllerClientTLSConfig_RequiresPublicHost(t *testing.T) {
	_, err := controllerClientTLSConfig("", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "public URL")

	_, err = controllerClientTLSConfig("https://", nil)
	require.Error(t, err)
}

func TestControllerClientTLSConfig_RejectsNonPEMCA(t *testing.T) {
	_, err := controllerClientTLSConfig("https://controller.example.com", []byte("not a certificate"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "ca.crt")
}

func testControllerCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "controller-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
