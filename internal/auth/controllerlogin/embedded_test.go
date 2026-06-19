package controllerlogin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedBootstrapLogin_Success(t *testing.T) {
	const wantToken = "access-token-abc"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/user/login":
			var req loginRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, "admin", req.Email)
			require.Equal(t, "ReplaceMe1!", req.Password)

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(loginResponse{
				AccessToken:  wantToken,
				RefreshToken: "refresh-token",
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL + "/api/v3")
	require.NoError(t, err)

	clt := iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
	require.NoError(t, EmbeddedBootstrapLogin(clt, "admin", "ReplaceMe1!"))
	require.Equal(t, wantToken, clt.GetAccessToken())
}

func TestEmbeddedBootstrapLogin_InvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/user/login":
			w.WriteHeader(http.StatusUnauthorized)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	baseURL, err := url.Parse(server.URL + "/api/v3")
	require.NoError(t, err)

	clt := iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
	err = EmbeddedBootstrapLogin(clt, "admin", "wrong")
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid credentials")
	require.NotContains(t, err.Error(), "wrong")
}

func TestUserLoginURL(t *testing.T) {
	got, err := userLoginURL("http://controller.cp-ns.svc.cluster.local:51121/api/v3")
	require.NoError(t, err)
	require.Equal(t, "http://controller.cp-ns.svc.cluster.local:51121/api/v3/user/login", got)
}
