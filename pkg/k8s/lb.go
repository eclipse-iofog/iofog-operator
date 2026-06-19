package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultPollInterval = 3 * time.Second

// ServiceGetter loads a Kubernetes Service by namespace and name.
// Implementations are provided for controller-runtime and client-go so the
// operator, iofogctl, and potctl can share LoadBalancer wait logic.
type ServiceGetter interface {
	GetService(ctx context.Context, namespace, name string) (*corev1.Service, error)
}

// ClientServiceGetter adapts a controller-runtime client.Reader.
type ClientServiceGetter struct {
	Client client.Reader
}

func (g ClientServiceGetter) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	svc := &corev1.Service{}
	err := g.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, svc)
	if err != nil {
		return nil, err
	}
	return svc, nil
}

// CoreV1ServiceGetter adapts client-go kubernetes.Interface for ctl tools.
type CoreV1ServiceGetter struct {
	Kube kubernetes.Interface
}

func (g CoreV1ServiceGetter) GetService(ctx context.Context, namespace, name string) (*corev1.Service, error) {
	return g.Kube.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
}

// LoadBalancerAddress returns the external IP or hostname from Service status.
// IP is preferred when both are set, matching historical go-sdk behavior.
func LoadBalancerAddress(svc *corev1.Service) (addr string, ok bool) {
	if svc == nil || len(svc.Status.LoadBalancer.Ingress) == 0 {
		return "", false
	}
	ing := svc.Status.LoadBalancer.Ingress[0]
	if ing.IP != "" {
		return ing.IP, true
	}
	if ing.Hostname != "" {
		return ing.Hostname, true
	}
	return "", false
}

// GetLoadBalancerAddress performs a non-blocking read of the Service LoadBalancer address.
func GetLoadBalancerAddress(ctx context.Context, getter ServiceGetter, namespace, name string) (addr string, ready bool, err error) {
	svc, err := getter.GetService(ctx, namespace, name)
	if err != nil {
		return "", false, err
	}
	addr, ready = LoadBalancerAddress(svc)
	return addr, ready, nil
}

// WaitLoadBalancerAddress polls until the Service has a LoadBalancer address or timeout elapses.
func WaitLoadBalancerAddress(ctx context.Context, getter ServiceGetter, namespace, name string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		addr, ready, err := GetLoadBalancerAddress(ctx, getter, namespace, name)
		if err != nil {
			return "", err
		}
		if ready {
			return addr, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("timed out waiting for LoadBalancer address on service %s/%s", namespace, name)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(defaultPollInterval):
		}
	}
}
