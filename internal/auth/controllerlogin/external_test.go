package controllerlogin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestExternalClientCredentialsLogin_Success(t *testing.T) {
	const wantToken = "external-access-token"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/realms/test/.well-known/openid-configuration":
			tokenEndpoint := requestURL(r, "/realms/test/protocol/openid-connect/token")
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(oidcConfiguration{
				TokenEndpoint: tokenEndpoint,
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/realms/test/protocol/openid-connect/token":
			require.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
			require.NoError(t, r.ParseForm())
			require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			require.Equal(t, "operator-client", r.Form.Get("client_id"))
			require.Equal(t, "s3cret!", r.Form.Get("client_secret"))

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(tokenResponse{AccessToken: wantToken}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(serverURL(server) + "/api/v3")
	require.NoError(t, err)

	clt := iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
	require.NoError(t, ExternalClientCredentialsLogin(
		clt,
		serverURL(server)+"/realms/test/",
		"operator-client",
		"s3cret!",
		"",
		true,
	))
	require.Equal(t, wantToken, clt.GetAccessToken())
}

func TestExternalClientCredentialsLogin_InvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/realms/test/.well-known/openid-configuration":
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(oidcConfiguration{
				TokenEndpoint: requestURL(r, "/realms/test/protocol/openid-connect/token"),
			}))
		case r.Method == http.MethodPost && r.URL.Path == "/realms/test/protocol/openid-connect/token":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(serverURL(server) + "/api/v3")
	require.NoError(t, err)

	clt := iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
	err = ExternalClientCredentialsLogin(clt, serverURL(server)+"/realms/test", "operator-client", "wrong", "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid client credentials")
	require.NotContains(t, err.Error(), "wrong")
}

func TestDiscoverTokenEndpoint_MissingTokenEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.Equal(t, "/realms/test/.well-known/openid-configuration", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	_, err := discoverTokenEndpoint(serverURL(server)+"/realms/test", true, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing token_endpoint")
}

func TestOpenIDConfigurationURL(t *testing.T) {
	got, err := openIDConfigurationURL("https://idp.example.com/realms/myrealm/")
	require.NoError(t, err)
	require.Equal(t, "https://idp.example.com/realms/myrealm/.well-known/openid-configuration", got)
}

func TestExternalClientCredentialsLogin_RejectsUntrustedTLS(t *testing.T) {
	posted := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		if r.URL.Path == "/api/v3/status" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	clt := iofogClientForIssuerTest(t, server.URL)
	err := ExternalClientCredentialsLogin(clt, server.URL+"/realms/test", "operator-client", "s3cret!", "", false)
	require.Error(t, err)
	require.False(t, posted)
}

func TestExternalClientCredentialsLogin_IssuerCaTrustsServer(t *testing.T) {
	const wantToken = "private-ca-token"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(oidcConfiguration{
				TokenEndpoint: requestURL(r, "/realms/test/protocol/openid-connect/token"),
			}))
		case r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(tokenResponse{AccessToken: wantToken}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	clt := iofogClientForIssuerTest(t, server.URL)
	require.NoError(t, ExternalClientCredentialsLogin(
		clt,
		server.URL+"/realms/test",
		"operator-client",
		"s3cret!",
		base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})),
		false,
	))
	require.Equal(t, wantToken, clt.GetAccessToken())
}

func TestExternalClientCredentialsLogin_DifferentCARejectsServer(t *testing.T) {
	posted := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		if r.URL.Path == "/api/v3/status" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	clt := iofogClientForIssuerTest(t, server.URL)
	err := ExternalClientCredentialsLogin(
		clt,
		server.URL+"/realms/test",
		"operator-client",
		"s3cret!",
		base64.StdEncoding.EncodeToString(testCAPEM(t, "other-ca")),
		false,
	)
	require.Error(t, err)
	require.False(t, posted)
}

func TestExternalClientCredentialsLogin_TokenEndpointHostMismatch(t *testing.T) {
	posted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posted = true
		}
		if r.URL.Path == "/api/v3/status" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(oidcConfiguration{
			TokenEndpoint: "http://evil.example/realms/test/protocol/openid-connect/token",
		}))
	}))
	t.Cleanup(server.Close)

	clt := iofogClientForIssuerTest(t, server.URL)
	err := ExternalClientCredentialsLogin(clt, server.URL+"/realms/test", "operator-client", "s3cret!", "", true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match issuer host")
	require.False(t, posted)
}

func TestIssuerTLSConfig_Invalid(t *testing.T) {
	_, err := issuerTLSConfig("not-base64")
	require.Error(t, err)
	require.Contains(t, err.Error(), "decode auth.issuerCa")

	_, err = issuerTLSConfig(base64.StdEncoding.EncodeToString([]byte("not a certificate")))
	require.Error(t, err)
	require.Contains(t, err.Error(), "PEM certificate")
}

func iofogClientForIssuerTest(t *testing.T, serverURL string) *iofogclient.Client {
	t.Helper()
	baseURL, err := url.Parse(strings.TrimSuffix(serverURL, "/") + "/api/v3")
	require.NoError(t, err)
	return iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
}

func testCAPEM(t *testing.T, commonName string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: commonName},
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

func serverURL(server *httptest.Server) string {
	return strings.TrimSuffix(server.URL, "/")
}

func requestURL(r *http.Request, path string) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host + path
}
