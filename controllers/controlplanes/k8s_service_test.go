package controllers

import (
	"context"
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/nats"
	"github.com/eclipse-iofog/iofog-operator/v3/controllers/controlplanes/router"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateService_PatchesExistingServiceType(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "cp-ns",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{{
				Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121),
			}},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := testControllerMicroservice("LoadBalancer", nil, "")

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: "controller"}, updated))
	require.Equal(t, corev1.ServiceTypeLoadBalancer, updated.Spec.Type)
}

func TestCreateService_PatchesConsolePort(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "controller",
			Namespace: "cp-ns",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121), Protocol: corev1.ProtocolTCP,
			}},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := testControllerMicroservice("LoadBalancer", nil, "")

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: "controller"}, updated))
	require.Len(t, updated.Spec.Ports, 2)
	require.Equal(t, controllerConsolePortName, updated.Spec.Ports[1].Name)
	require.Equal(t, int32(controllerConsoleServicePort), updated.Spec.Ports[1].Port)
	require.Equal(t, int32(defaultControllerConsolePort), updated.Spec.Ports[1].TargetPort.IntVal)
}

func TestCreateService_PatchesAnnotationsAndExternalTrafficPolicy(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "controller",
			Namespace:   "cp-ns",
			Annotations: map[string]string{"old": "value"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121),
			}},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := testControllerMicroservice(
		"LoadBalancer",
		map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"},
		"Local",
	)

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: "controller"}, updated))
	require.Equal(t, map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"}, updated.Annotations)
	require.Equal(t, corev1.ServiceExternalTrafficPolicyTypeLocal, updated.Spec.ExternalTrafficPolicy)
}

func TestCreateService_PatchesRouterServiceAnnotations(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "router",
			Namespace:   "cp-ns",
			Annotations: map[string]string{"old": "value"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "router-message", Port: int32(router.MessagePort), TargetPort: intstr.FromInt(router.MessagePort)},
				{Name: "router-interior", Port: int32(router.InteriorPort), TargetPort: intstr.FromInt(router.InteriorPort)},
				{Name: "router-edge", Port: int32(router.EdgePort), TargetPort: intstr.FromInt(router.EdgePort)},
			},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := newRouterMicroservice(routerMicroserviceConfig{
		serviceType:        "LoadBalancer",
		serviceAnnotations: map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"},
	})

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: "router"}, updated))
	require.Equal(t, map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"}, updated.Annotations)
}

func TestCreateService_PatchesNatsClientServiceAnnotations(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        nats.ClientServiceName,
			Namespace:   "cp-ns",
			Annotations: map[string]string{"old": "value"},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "cluster", Port: int32(nats.DefaultClusterPort), TargetPort: intstr.FromInt(nats.DefaultClusterPort)},
				{Name: "leaf", Port: int32(nats.DefaultLeafPort), TargetPort: intstr.FromInt(nats.DefaultLeafPort)},
				{Name: "mqtt", Port: int32(nats.DefaultMqttPort), TargetPort: intstr.FromInt(nats.DefaultMqttPort)},
			},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := newNatsMicroservice(natsMicroserviceConfig{
		replicas:           2,
		storageSize:        nats.DefaultStorageSizePVC,
		jetStreamKeySecret: nats.JetStreamKeySecretName("test-cp"),
		serviceType:        "LoadBalancer",
		serviceAnnotations: map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"},
	})

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: nats.ClientServiceName}, updated))
	require.Equal(t, map[string]string{"service.beta.kubernetes.io/aws-load-balancer-type": "nlb"}, updated.Annotations)
}

func TestCreateService_PatchesNatsServerServiceType(t *testing.T) {
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nats.ServerServiceName,
			Namespace: "cp-ns",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{Name: "client", Port: int32(nats.DefaultServerPort), TargetPort: intstr.FromInt(nats.DefaultServerPort)},
				{Name: "monitor", Port: int32(nats.DefaultHttpPort), TargetPort: intstr.FromInt(nats.DefaultHttpPort)},
			},
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := newNatsMicroservice(natsMicroserviceConfig{
		replicas:           2,
		storageSize:        nats.DefaultStorageSizePVC,
		jetStreamKeySecret: nats.JetStreamKeySecretName("test-cp"),
		serverServiceType:  "LoadBalancer",
	})

	require.NoError(t, r.createService(context.Background(), ms))

	updated := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: nats.ServerServiceName}, updated))
	require.Equal(t, corev1.ServiceTypeLoadBalancer, updated.Spec.Type)
}

func TestCreateService_SkipsUpdateWhenUnchanged(t *testing.T) {
	annotations := map[string]string{"keep": "me"}
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "controller",
			Namespace:       "cp-ns",
			Annotations:     annotations,
			ResourceVersion: "42",
		},
		Spec: corev1.ServiceSpec{
			Type:                  corev1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: corev1.ServiceExternalTrafficPolicyTypeLocal,
			Ports:                 controllerServicePortsFixture(),
		},
	}

	r := newServiceTestReconciler(t, existing)
	ms := testControllerMicroservice("LoadBalancer", annotations, "Local")

	require.NoError(t, r.createService(context.Background(), ms))

	unchanged := &corev1.Service{}
	require.NoError(t, r.Client.Get(context.Background(), types.NamespacedName{Namespace: "cp-ns", Name: "controller"}, unchanged))
	require.Equal(t, "42", unchanged.ResourceVersion)
}

func newServiceTestReconciler(t *testing.T, objects ...runtime.Object) *ControlPlaneReconciler {
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

func testControllerMicroservice(serviceType string, annotations map[string]string, trafficPolicy string) *microservice {
	return &microservice{
		name:   "controller",
		labels: map[string]string{"app": "controller"},
		services: []service{{
			name:               "controller",
			serviceType:        serviceType,
			serviceAnnotations: annotations,
			trafficPolicy:      trafficPolicy,
			ports:              controllerServicePortsFixture(),
		}},
	}
}

func controllerServicePortsFixture() []corev1.ServicePort {
	return []corev1.ServicePort{
		{Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121), Protocol: corev1.ProtocolTCP},
		{Name: controllerConsolePortName, Port: controllerConsoleServicePort, TargetPort: intstr.FromInt(defaultControllerConsolePort), Protocol: corev1.ProtocolTCP},
	}
}
