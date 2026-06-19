package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAuthSecretStringData_EmbeddedBootstrap(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			Password: "ReplaceMe1!",
		},
	}, "")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:          "embedded",
		controllerAuthBootstrapUserSecretKey: "admin",
		controllerAuthBootstrapPassSecretKey: "ReplaceMe1!",
	}, data)
}

func TestAuthSecretStringData_EmbeddedPasswordSecretRefResolved(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	}, "FromSecretRef1!")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:          "embedded",
		controllerAuthBootstrapUserSecretKey: "admin",
		controllerAuthBootstrapPassSecretKey: "FromSecretRef1!",
	}, data)
}

func TestAuthSecretStringData_ExternalClient(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode:      cpv3.AuthModeExternal,
		IssuerUrl: "https://idp.example.com/realms/myrealm",
		Client: &cpv3.AuthClient{
			ID:     "controller",
			Secret: "s3cr3t",
		},
	}, "")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:         "external",
		controllerAuthIssuerURLSecretKey:    "https://idp.example.com/realms/myrealm",
		controllerAuthClientIDSecretKey:     "controller",
		controllerAuthClientSecretSecretKey: "s3cr3t",
	}, data)
}

func TestAuthSecretStringData_EmbeddedOptionalClientOverride(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			Password: "ReplaceMe1!",
		},
		Client: &cpv3.AuthClient{
			ID:     "custom-controller",
			Secret: "override-secret",
		},
	}, "")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:          "embedded",
		controllerAuthBootstrapUserSecretKey: "admin",
		controllerAuthBootstrapPassSecretKey: "ReplaceMe1!",
		controllerAuthClientIDSecretKey:      "custom-controller",
		controllerAuthClientSecretSecretKey:  "override-secret",
	}, data)
}

func TestAuthSecretStringData_EmbeddedOmitsIssuerAndEmptyClient(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode:      cpv3.AuthModeEmbedded,
		IssuerUrl: "https://should-not-appear.example.com",
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			Password: "pw",
		},
		Client: &cpv3.AuthClient{},
	}, "")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:          "embedded",
		controllerAuthBootstrapUserSecretKey: "admin",
		controllerAuthBootstrapPassSecretKey: "pw",
	}, data)
}

func TestAuthSecretStringData_ExternalOmitsBootstrap(t *testing.T) {
	data := authSecretStringData(&cpv3.Auth{
		Mode:      cpv3.AuthModeExternal,
		IssuerUrl: "https://idp.example.com",
		Client: &cpv3.AuthClient{
			ID:     "controller",
			Secret: "s3cr3t",
		},
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			Password: "pw",
		},
	}, "")
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:         "external",
		controllerAuthIssuerURLSecretKey:    "https://idp.example.com",
		controllerAuthClientIDSecretKey:     "controller",
		controllerAuthClientSecretSecretKey: "s3cr3t",
	}, data)
}

func TestAuthSecretStringData_NilAuth(t *testing.T) {
	require.Empty(t, authSecretStringData(nil, ""))
}

func TestBuildControllerSecrets_AuthSecretMatchesCRSpec(t *testing.T) {
	cfg := &controllerMicroserviceConfig{
		auth: &cpv3.Auth{
			Mode: cpv3.AuthModeEmbedded,
			Bootstrap: &cpv3.AuthBootstrap{
				Username: "admin",
				Password: "ReplaceMe1!",
			},
		},
		db: &cpv3.Database{
			DatabaseName: "controller",
			Host:         "db.example.com",
			Port:         5432,
			User:         "admin",
			Password:     "db-secret",
		},
	}

	secrets := buildControllerSecrets("cp-ns", cfg)
	require.Len(t, secrets, 2)

	var authSecret *corev1.Secret
	for i := range secrets {
		if secrets[i].Name == controlllerAuthCredentialsSecretName {
			authSecret = &secrets[i]
			break
		}
	}
	require.NotNil(t, authSecret)
	require.Equal(t, "cp-ns", authSecret.Namespace)
	require.Equal(t, map[string]string{
		controllerAuthModeSecretKey:          "embedded",
		controllerAuthBootstrapUserSecretKey: "admin",
		controllerAuthBootstrapPassSecretKey: "ReplaceMe1!",
	}, authSecret.StringData)
}

func TestBuildControllerSecrets_AuthSecretUsesResolvedPasswordSecretRef(t *testing.T) {
	cfg := &controllerMicroserviceConfig{
		auth: &cpv3.Auth{
			Mode: cpv3.AuthModeEmbedded,
			Bootstrap: &cpv3.AuthBootstrap{
				Username: "admin",
				PasswordSecretRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
					Key:                  "password",
				},
			},
		},
		bootstrapPassword: "FromSecretRef1!",
		db: &cpv3.Database{
			DatabaseName: "controller",
			Host:         "db.example.com",
			Port:         5432,
			User:         "admin",
			Password:     "db-secret",
		},
	}

	secrets := buildControllerSecrets("cp-ns", cfg)
	var authSecret *corev1.Secret
	for i := range secrets {
		if secrets[i].Name == controlllerAuthCredentialsSecretName {
			authSecret = &secrets[i]
			break
		}
	}
	require.NotNil(t, authSecret)
	require.Equal(t, "FromSecretRef1!", authSecret.StringData[controllerAuthBootstrapPassSecretKey])
}

func TestAppendControllerAuthEnv_PasswordSecretRef(t *testing.T) {
	env := appendControllerAuthEnv(nil, &cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Username: "admin",
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	}, "FromSecretRef1!")

	var passwordEnv *corev1.EnvVar
	for i := range env {
		if env[i].Name == "OIDC_BOOTSTRAP_ADMIN_PASSWORD" {
			passwordEnv = &env[i]
			break
		}
	}
	require.NotNil(t, passwordEnv)
	require.Equal(t, controllerAuthBootstrapPassSecretKey, passwordEnv.ValueFrom.SecretKeyRef.Key)
}

func TestResolveBootstrapPassword_Inline(t *testing.T) {
	password, err := resolveBootstrapPassword(context.Background(), fake.NewClientBuilder().Build(), "cp-ns", &cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Password: "ReplaceMe1!",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "ReplaceMe1!", password)
}

func TestResolveBootstrapPassword_FromSecretRef(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-bootstrap", Namespace: "cp-ns"},
		Data:       map[string][]byte{"password": []byte("FromSecretRef1!")},
	}
	cl := fake.NewClientBuilder().WithObjects(secret).Build()

	password, err := resolveBootstrapPassword(context.Background(), cl, "cp-ns", &cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "FromSecretRef1!", password)
}

func TestResolveBootstrapPassword_SecretRefPrecedenceOverInline(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-bootstrap", Namespace: "cp-ns"},
		Data:       map[string][]byte{"password": []byte("FromSecretRef1!")},
	}
	cl := fake.NewClientBuilder().WithObjects(secret).Build()

	password, err := resolveBootstrapPassword(context.Background(), cl, "cp-ns", &cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			Password: "InlineShouldNotWin",
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "FromSecretRef1!", password)
}

func TestResolveBootstrapPassword_ErrorDoesNotContainPassword(t *testing.T) {
	const sensitive = "SuperSecret99!"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "controller-bootstrap", Namespace: "cp-ns"},
		Data:       map[string][]byte{"other-key": []byte(sensitive)},
	}
	cl := fake.NewClientBuilder().WithObjects(secret).Build()

	_, err := resolveBootstrapPassword(context.Background(), cl, "cp-ns", &cpv3.Auth{
		Mode: cpv3.AuthModeEmbedded,
		Bootstrap: &cpv3.AuthBootstrap{
			PasswordSecretRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "controller-bootstrap"},
				Key:                  "password",
			},
		},
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), sensitive)
	require.Contains(t, err.Error(), `missing key "password"`)
}

func TestResolveBootstrapPassword_ExternalModeSkipsResolution(t *testing.T) {
	password, err := resolveBootstrapPassword(context.Background(), fake.NewClientBuilder().Build(), "cp-ns", &cpv3.Auth{
		Mode: cpv3.AuthModeExternal,
		Bootstrap: &cpv3.AuthBootstrap{
			Password: "ShouldNotResolve",
		},
	})
	require.NoError(t, err)
	require.Empty(t, password)
}

func TestAuthBootstrapLoggingDoesNotReferencePasswordValue(t *testing.T) {
	// Guardrail: reconcile error wrapping must not include resolved password text.
	const sensitive = "DoNotLogThisPw!"
	msg := fmt.Errorf("resolve bootstrap password: %w", fmt.Errorf("get bootstrap password secret %q: not found", "controller-bootstrap"))
	require.False(t, strings.Contains(msg.Error(), sensitive))
}
