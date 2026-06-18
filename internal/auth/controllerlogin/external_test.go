package controllerlogin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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
	err = ExternalClientCredentialsLogin(clt, serverURL(server)+"/realms/test", "operator-client", "wrong")
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

	_, err := discoverTokenEndpoint(serverURL(server) + "/realms/test")
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing token_endpoint")
}

func TestOpenIDConfigurationURL(t *testing.T) {
	got, err := openIDConfigurationURL("https://idp.example.com/realms/myrealm/")
	require.NoError(t, err)
	require.Equal(t, "https://idp.example.com/realms/myrealm/.well-known/openid-configuration", got)
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
