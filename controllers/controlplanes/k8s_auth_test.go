package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	iofogclient "github.com/eclipse-iofog/iofog-go-sdk/v3/pkg/client"
	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLoginIofogClient_EmbeddedSuccess(t *testing.T) {
	const (
		username    = "admin"
		password    = "ReplaceMe1!"
		accessToken = "embedded-access-token"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/user/login":
			var req struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, username, req.Email)
			require.Equal(t, password, req.Password)
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{
				"accessToken": accessToken,
			}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	r := newLoginTestReconciler(t, cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: username,
			Password: password,
		},
	})
	clt := iofogClientForTest(t, server.URL)

	require.NoError(t, r.loginIofogClient(context.Background(), clt))
	require.Equal(t, accessToken, clt.GetAccessToken())
}

func TestLoginIofogClient_EmbeddedPasswordSecretRef(t *testing.T) {
	const (
		username    = "admin"
		password    = "FromSecretRef1!"
		accessToken = "embedded-secretref-token"
	)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-bootstrap", Namespace: "pot-ns"},
		Data:       map[string][]byte{"password": []byte(password)},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v3/user/login":
			var req struct {
				Password string `json:"password"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, password, req.Password)
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"accessToken": accessToken}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	r := newLoginTestReconciler(t, cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: username,
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	}, secret)
	clt := iofogClientForTest(t, server.URL)

	require.NoError(t, r.loginIofogClient(context.Background(), clt))
	require.Equal(t, accessToken, clt.GetAccessToken())
}

func TestLoginIofogClient_ExternalSuccess(t *testing.T) {
	const (
		clientID     = "operator-client"
		clientSecret = "s3cret!"
		accessToken  = "external-access-token"
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/realms/test/.well-known/openid-configuration":
			tokenEndpoint := "http://" + r.Host + "/realms/test/protocol/openid-connect/token"
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"token_endpoint": tokenEndpoint}))
		case r.Method == http.MethodPost && r.URL.Path == "/realms/test/protocol/openid-connect/token":
			require.NoError(t, r.ParseForm())
			require.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			require.Equal(t, clientID, r.Form.Get("client_id"))
			require.Equal(t, clientSecret, r.Form.Get("client_secret"))
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"access_token": accessToken}))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(server.Close)

	r := newLoginTestReconciler(t, cpv3.Auth{
		Mode:      cpv3.AuthModeExternal,
		IssuerUrl: server.URL + "/realms/test/",
		Client: &cpv3.AuthClient{
			ID:     clientID,
			Secret: clientSecret,
		},
	})
	clt := iofogClientForTest(t, server.URL)

	require.NoError(t, r.loginIofogClient(context.Background(), clt))
	require.Equal(t, accessToken, clt.GetAccessToken())
}

func TestLoginIofogClient_AuthModeSelectionErrors(t *testing.T) {
	const sensitivePassword = "DoNotLeakThisPw!"
	const sensitiveSecret = "DoNotLeakThisSecret!"

	tests := []struct {
		name      string
		auth      cpv3.Auth
		objects   []client.Object
		wantSubstr string
	}{
		{
			name:       "embedded missing bootstrap",
			auth:       cpv3.Auth{Mode: cpv3.AuthModeEmbedded},
			wantSubstr: "auth.bootstrap is required when mode=embedded",
		},
		{
			name: "embedded missing username",
			auth: cpv3.Auth{
				Mode:      cpv3.AuthModeEmbedded,
				Bootstrap: &cpv3.AuthBootstrap{Password: sensitivePassword},
			},
			wantSubstr: "auth.bootstrap.username is required when mode=embedded",
		},
		{
			name: "embedded missing password",
			auth: cpv3.Auth{
				Mode: cpv3.AuthModeEmbedded,
				Bootstrap: &cpv3.AuthBootstrap{
					Username: "admin",
				},
			},
			wantSubstr: "auth.bootstrap password is required when mode=embedded",
		},
		{
			name: "embedded secret ref not found",
			auth: cpv3.Auth{
				Mode: cpv3.AuthModeEmbedded,
				Bootstrap: &cpv3.AuthBootstrap{
					Username: "admin",
					PasswordSecretRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "missing-secret"},
						Key:                  "password",
					},
				},
			},
			wantSubstr: "resolve bootstrap password",
		},
		{
			name:       "external missing issuer",
			auth:       cpv3.Auth{Mode: cpv3.AuthModeExternal},
			wantSubstr: "auth.issuerUrl is required when mode=external",
		},
		{
			name: "external missing client credentials",
			auth: cpv3.Auth{
				Mode:      cpv3.AuthModeExternal,
				IssuerUrl: "https://idp.example.com/realms/test",
				Client:    &cpv3.AuthClient{ID: "operator-client"},
			},
			wantSubstr: "auth.client id and secret are required when mode=external",
		},
		{
			name:       "unsupported mode",
			auth:       cpv3.Auth{Mode: "legacy"},
			wantSubstr: `unsupported auth mode: "legacy"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newLoginTestReconciler(t, tt.auth, tt.objects...)
			clt := iofogClientForTest(t, "http://unused.example.com")

			err := r.loginIofogClient(context.Background(), clt)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantSubstr)
			require.NotContains(t, err.Error(), sensitivePassword)
			require.NotContains(t, err.Error(), sensitiveSecret)
		})
	}
}

func TestLoginIofogClient_InvalidCredentialsDoNotLeakSecrets(t *testing.T) {
	const password = "SuperSecretPw!"
	const clientSecret = "SuperSecretClient!"

	t.Run("embedded", func(t *testing.T) {
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

		r := newLoginTestReconciler(t, cpv3.Auth{
			Mode: cpv3.AuthModeEmbedded,
			Bootstrap: &cpv3.AuthBootstrap{
				Username: "admin",
				Password: password,
			},
		})
		clt := iofogClientForTest(t, server.URL)

		err := r.loginIofogClient(context.Background(), clt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid credentials")
		require.NotContains(t, err.Error(), password)
	})

	t.Run("external", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/api/v3/status":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"versions":{"controller":"3.8.0"}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/realms/test/.well-known/openid-configuration":
				tokenEndpoint := "http://" + r.Host + "/realms/test/protocol/openid-connect/token"
				w.Header().Set("Content-Type", "application/json")
				require.NoError(t, json.NewEncoder(w).Encode(map[string]string{"token_endpoint": tokenEndpoint}))
			case r.Method == http.MethodPost && r.URL.Path == "/realms/test/protocol/openid-connect/token":
				w.WriteHeader(http.StatusUnauthorized)
			default:
				t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
			}
		}))
		t.Cleanup(server.Close)

		r := newLoginTestReconciler(t, cpv3.Auth{
			Mode:      cpv3.AuthModeExternal,
			IssuerUrl: server.URL + "/realms/test",
			Client: &cpv3.AuthClient{
				ID:     "operator-client",
				Secret: clientSecret,
			},
		})
		clt := iofogClientForTest(t, server.URL)

		err := r.loginIofogClient(context.Background(), clt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid client credentials")
		require.NotContains(t, err.Error(), clientSecret)
	})
}

func newLoginTestReconciler(t *testing.T, auth cpv3.Auth, objects ...client.Object) *ControlPlaneReconciler {
	t.Helper()

	builder := fake.NewClientBuilder()
	if len(objects) > 0 {
		builder = builder.WithObjects(objects...)
	}

	return &ControlPlaneReconciler{
		Client: builder.Build(),
		cp: cpv3.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Namespace: "pot-ns", Name: "test-cp"},
			Spec:       cpv3.ControlPlaneSpec{Auth: auth},
		},
	}
}

func iofogClientForTest(t *testing.T, serverURL string) *iofogclient.Client {
	t.Helper()

	baseURL, err := url.Parse(strings.TrimSuffix(serverURL, "/") + "/api/v3")
	require.NoError(t, err)
	return iofogclient.New(iofogclient.Options{BaseURL: baseURL, Timeout: 5})
}
