package controllers

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestServicePortsEqual(t *testing.T) {
	api := corev1.ServicePort{Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121), Protocol: corev1.ProtocolTCP}
	console := corev1.ServicePort{Name: controllerConsolePortName, Port: controllerConsoleServicePort, TargetPort: intstr.FromInt(defaultControllerConsolePort), Protocol: corev1.ProtocolTCP}

	require.True(t, servicePortsEqual([]corev1.ServicePort{api, console}, []corev1.ServicePort{api, console}))
	require.False(t, servicePortsEqual([]corev1.ServicePort{api}, []corev1.ServicePort{api, console}))
	require.False(t, servicePortsEqual(
		[]corev1.ServicePort{{Name: controllerConsolePortName, Port: 80, TargetPort: intstr.FromInt(9000), Protocol: corev1.ProtocolTCP}},
		[]corev1.ServicePort{console},
	))
}

func TestServiceNeedsPatch_Ports(t *testing.T) {
	existing := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{{
				Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121), Protocol: corev1.ProtocolTCP,
			}},
		},
	}
	desired := &corev1.Service{
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{Name: "controller-api", Port: 51121, TargetPort: intstr.FromInt(51121), Protocol: corev1.ProtocolTCP},
				{Name: controllerConsolePortName, Port: controllerConsoleServicePort, TargetPort: intstr.FromInt(defaultControllerConsolePort), Protocol: corev1.ProtocolTCP},
			},
		},
	}
	require.True(t, serviceNeedsPatch(existing, desired))

	applyServicePatch(existing, desired)
	require.Len(t, existing.Spec.Ports, 2)
	require.Equal(t, controllerConsolePortName, existing.Spec.Ports[1].Name)
}

func TestNewControllerIngress_ExposesConsoleAndAPIBackends(t *testing.T) {
	ing := newControllerIngress("pot-ns", "test-cp", &controllerIngressConfig{host: "pot.local"})
	require.Len(t, ing.Spec.Rules, 1)
	paths := ing.Spec.Rules[0].HTTP.Paths
	require.Len(t, paths, 2)

	require.Equal(t, "/", paths[0].Path)
	require.Equal(t, controllerConsolePortName, paths[0].Backend.Service.Port.Name)

	require.Equal(t, "/api/v3", paths[1].Path)
	require.Equal(t, "controller-api", paths[1].Backend.Service.Port.Name)
}

func TestIngressRulesEqual_DetectsLegacyConsoleBackend(t *testing.T) {
	desired := newControllerIngress("pot-ns", "test-cp", &controllerIngressConfig{host: "pot.local"})
	legacy := desired.DeepCopy()
	legacy.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Port.Name = "ecn-viewer"

	require.False(t, ingressRulesEqual(legacy.Spec.Rules, desired.Spec.Rules))
	require.True(t, ingressRulesEqual(desired.Spec.Rules, desired.Spec.Rules))
}
