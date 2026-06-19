package controllers

import (
	"context"
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateIngress_PatchesHostClassTLSAndAnnotations(t *testing.T) {
	classA := "nginx-a"
	classB := "nginx-b"
	existing := newControllerIngress("cp-ns", "test-cp", &controllerIngressConfig{
		annotations:      map[string]string{"old": "value"},
		ingressClassName: classA,
		host:             "old.example.com",
		secretName:       "old-tls",
	})
	existing.ResourceVersion = "1"

	r := newIngressTestReconciler(t, existing)
	cfg := &controllerIngressConfig{
		annotations:      map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"},
		ingressClassName: classB,
		host:             "new.example.com",
		secretName:       "new-tls",
	}

	require.NoError(t, r.createIngress(context.Background(), cfg))

	updated := &networkingv1.Ingress{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: controllerIngressName}, updated))
	require.Equal(t, map[string]string{"cert-manager.io/cluster-issuer": "letsencrypt"}, updated.Annotations)
	require.Equal(t, &classB, updated.Spec.IngressClassName)
	require.Equal(t, "new-tls", updated.Spec.TLS[0].SecretName)
	require.Equal(t, []string{"new.example.com"}, updated.Spec.TLS[0].Hosts)
	require.Equal(t, "new.example.com", updated.Spec.Rules[0].Host)
	require.Len(t, updated.Spec.Rules[0].HTTP.Paths, 2)
	require.Equal(t, controllerConsolePortName, updated.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name)
	require.Equal(t, "controller-api", updated.Spec.Rules[0].HTTP.Paths[1].Backend.Service.Port.Name)
}

func TestCreateIngress_PatchesLegacyConsoleBackend(t *testing.T) {
	legacy := newControllerIngress("cp-ns", "test-cp", &controllerIngressConfig{
		host:       "ctrl.example.com",
		secretName: "ctrl-tls",
	})
	legacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name = "ecn-viewer"
	legacy.ResourceVersion = "1"

	r := newIngressTestReconciler(t, legacy)
	cfg := &controllerIngressConfig{
		host:       "ctrl.example.com",
		secretName: "ctrl-tls",
	}

	require.NoError(t, r.createIngress(context.Background(), cfg))

	updated := &networkingv1.Ingress{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: controllerIngressName}, updated))
	require.Equal(t, controllerConsolePortName, updated.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name)
}

func TestCreateIngress_SkipsUpdateWhenUnchanged(t *testing.T) {
	class := "nginx"
	annotations := map[string]string{"keep": "me"}
	cfg := &controllerIngressConfig{
		annotations:      annotations,
		ingressClassName: class,
		host:             "ctrl.example.com",
		secretName:       "ctrl-tls",
	}
	existing := newControllerIngress("cp-ns", "test-cp", cfg)
	existing.ResourceVersion = "42"

	r := newIngressTestReconciler(t, existing)
	require.NoError(t, r.createIngress(context.Background(), cfg))

	unchanged := &networkingv1.Ingress{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: controllerIngressName}, unchanged))
	require.Equal(t, "42", unchanged.ResourceVersion)
}

func newIngressTestReconciler(t *testing.T, objects ...runtime.Object) *ControlPlaneReconciler {
	t.Helper()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cpv3.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objects) > 0 {
		builder = builder.WithRuntimeObjects(objects...)
	}

	return &ControlPlaneReconciler{
		Client: builder.Build(),
		Scheme: scheme,
		cp: cpv3.ControlPlane{
			ObjectMeta: metav1.ObjectMeta{Namespace: "cp-ns", Name: "test-cp"},
		},
	}
}
