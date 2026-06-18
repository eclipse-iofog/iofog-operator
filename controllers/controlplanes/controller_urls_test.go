package controllers

import (
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestResolveControllerAccess_IngressDefaults(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeClusterIP)},
		},
		Ingresses: cpv3.Ingresses{
			Controller: cpv3.ControllerIngress{Host: "pot.local"},
		},
		Controller: cpv3.Controller{},
	}

	access, needLB := resolveControllerAccess(spec, "")
	require.False(t, needLB)
	require.Equal(t, "http://pot.local", access.PublicURL)
	require.Equal(t, "http://pot.local", access.ConsoleURL)
	require.NotNil(t, access.TrustProxy)
	require.True(t, *access.TrustProxy)
}

func TestResolveControllerAccess_IngressHTTPSFromTLSSecret(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeClusterIP)},
		},
		Ingresses: cpv3.Ingresses{
			Controller: cpv3.ControllerIngress{Host: "pot.local", SecretName: "pot-tls"},
		},
		Controller: cpv3.Controller{Https: ptr.To(false)},
	}

	access, _ := resolveControllerAccess(spec, "")
	require.Equal(t, "https://pot.local", access.PublicURL)
}

func TestResolveControllerAccess_IngressRespectsExplicitTrustProxy(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeClusterIP)},
		},
		Ingresses: cpv3.Ingresses{
			Controller: cpv3.ControllerIngress{Host: "pot.local"},
		},
		Controller: cpv3.Controller{TrustProxy: ptr.To(false)},
	}

	access, _ := resolveControllerAccess(spec, "")
	require.Nil(t, access.TrustProxy)
}

func TestResolveControllerAccess_LoadBalancerDefaults(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeLoadBalancer)},
		},
		Controller: cpv3.Controller{},
	}

	access, needLB := resolveControllerAccess(spec, "192.168.1.10")
	require.False(t, needLB)
	require.Equal(t, "http://192.168.1.10:51121", access.PublicURL)
	require.Equal(t, "http://192.168.1.10", access.ConsoleURL)
}

func TestResolveControllerAccess_LoadBalancerNeedsIP(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeLoadBalancer)},
		},
		Controller: cpv3.Controller{},
	}

	_, needLB := resolveControllerAccess(spec, "")
	require.True(t, needLB)
}

func TestResolveControllerAccess_PublicURLSetsConsoleURL(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeLoadBalancer)},
		},
		Controller: cpv3.Controller{PublicUrl: "https://ctrl.example.com"},
	}

	access, needLB := resolveControllerAccess(spec, "")
	require.False(t, needLB)
	require.Equal(t, "https://ctrl.example.com", access.PublicURL)
	require.Equal(t, "https://ctrl.example.com", access.ConsoleURL)
}

func TestResolveControllerAccess_ExplicitURLsUnchanged(t *testing.T) {
	spec := cpv3.ControlPlaneSpec{
		Services: cpv3.Services{
			Controller: cpv3.Service{Type: string(corev1.ServiceTypeLoadBalancer)},
		},
		Controller: cpv3.Controller{
			PublicUrl:  "https://api.example.com",
			ConsoleUrl: "https://ui.example.com",
		},
	}

	access, _ := resolveControllerAccess(spec, "10.0.0.1")
	require.Equal(t, "https://api.example.com", access.PublicURL)
	require.Equal(t, "https://ui.example.com", access.ConsoleURL)
}
