package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stubServiceGetter struct {
	svc *corev1.Service
	err error
}

func (s stubServiceGetter) GetService(_ context.Context, _, _ string) (*corev1.Service, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.svc, nil
}

func TestLoadBalancerAddress(t *testing.T) {
	t.Run("empty status", func(t *testing.T) {
		addr, ok := LoadBalancerAddress(&corev1.Service{})
		require.False(t, ok)
		require.Empty(t, addr)
	})

	t.Run("IP", func(t *testing.T) {
		svc := &corev1.Service{
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{IP: "10.0.0.1", Hostname: "lb.example.com"}},
				},
			},
		}
		addr, ok := LoadBalancerAddress(svc)
		require.True(t, ok)
		require.Equal(t, "10.0.0.1", addr)
	})

	t.Run("hostname only", func(t *testing.T) {
		svc := &corev1.Service{
			Status: corev1.ServiceStatus{
				LoadBalancer: corev1.LoadBalancerStatus{
					Ingress: []corev1.LoadBalancerIngress{{Hostname: "elb.amazonaws.com"}},
				},
			},
		}
		addr, ok := LoadBalancerAddress(svc)
		require.True(t, ok)
		require.Equal(t, "elb.amazonaws.com", addr)
	})
}

func TestGetLoadBalancerAddress(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "router", Namespace: "ns"},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "192.168.1.5"}},
			},
		},
	}
	addr, ready, err := GetLoadBalancerAddress(context.Background(), stubServiceGetter{svc: svc}, "ns", "router")
	require.NoError(t, err)
	require.True(t, ready)
	require.Equal(t, "192.168.1.5", addr)
}

func TestWaitLoadBalancerAddress(t *testing.T) {
	svc := &corev1.Service{
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{{IP: "10.1.2.3"}},
			},
		},
	}
	addr, err := WaitLoadBalancerAddress(context.Background(), stubServiceGetter{svc: svc}, "ns", "nats", time.Second)
	require.NoError(t, err)
	require.Equal(t, "10.1.2.3", addr)
}

func TestWaitLoadBalancerAddress_timeout(t *testing.T) {
	_, err := WaitLoadBalancerAddress(context.Background(), stubServiceGetter{svc: &corev1.Service{}}, "ns", "nats", 10*time.Millisecond)
	require.Error(t, err)
	require.Contains(t, err.Error(), "timed out waiting for LoadBalancer address")
}
