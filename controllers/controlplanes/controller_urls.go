package controllers

import (
	"fmt"
	"strings"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	corev1 "k8s.io/api/core/v1"
)

// controllerAccessURLs holds resolved controller external URLs and optional auto trustProxy.
type controllerAccessURLs struct {
	PublicURL  string
	ConsoleURL string
	TrustProxy *bool
}

func ingressProvided(spec cpv3.ControlPlaneSpec) bool {
	return strings.EqualFold(spec.Services.Controller.Type, string(corev1.ServiceTypeClusterIP)) &&
		spec.Ingresses.Controller.Host != ""
}

func controllerHTTPSFromSpec(spec cpv3.ControlPlaneSpec) bool {
	return spec.Controller.Https != nil && *spec.Controller.Https
}

func ingressURLScheme(spec cpv3.ControlPlaneSpec) string {
	if controllerHTTPSFromSpec(spec) || spec.Ingresses.Controller.SecretName != "" {
		return "https"
	}
	return "http"
}

func loadBalancerURLScheme(spec cpv3.ControlPlaneSpec) string {
	if controllerHTTPSFromSpec(spec) {
		return "https"
	}
	return "http"
}

func controllerAccessNeedsLBIP(spec cpv3.ControlPlaneSpec) bool {
	if ingressProvided(spec) {
		return false
	}
	if !strings.EqualFold(spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) {
		return false
	}
	return spec.Controller.PublicUrl == ""
}

// resolveControllerAccess computes publicUrl, consoleUrl, and optional trustProxy defaults.
// lbIP is the controller Service status.loadBalancer.ingress[0].ip (empty if not yet assigned).
// needLBIP is true when reconcile must wait for a LoadBalancer IP before deploying the controller.
func resolveControllerAccess(spec cpv3.ControlPlaneSpec, lbIP string) (controllerAccessURLs, bool) {
	out := controllerAccessURLs{
		PublicURL:  spec.Controller.PublicUrl,
		ConsoleURL: spec.Controller.ConsoleUrl,
	}

	if out.PublicURL != "" && out.ConsoleURL == "" {
		out.ConsoleURL = out.PublicURL
	}

	if ingressProvided(spec) {
		if out.PublicURL == "" {
			out.PublicURL = fmt.Sprintf("%s://%s", ingressURLScheme(spec), spec.Ingresses.Controller.Host)
		}
		if out.ConsoleURL == "" {
			out.ConsoleURL = out.PublicURL
		}
		if spec.Controller.TrustProxy == nil {
			t := true
			out.TrustProxy = &t
		}
		return out, false
	}

	if controllerAccessNeedsLBIP(spec) && lbIP == "" {
		return out, true
	}

	if strings.EqualFold(spec.Services.Controller.Type, string(corev1.ServiceTypeLoadBalancer)) && lbIP != "" {
		scheme := loadBalancerURLScheme(spec)
		if out.PublicURL == "" {
			out.PublicURL = fmt.Sprintf("%s://%s:%d", scheme, lbIP, controllerAPIPort)
		}
		if out.ConsoleURL == "" {
			out.ConsoleURL = fmt.Sprintf("%s://%s", scheme, lbIP)
		}
	}

	return out, false
}

func applyControllerAccess(config *controllerMicroserviceConfig, access controllerAccessURLs, specTrustProxy *bool) {
	if access.PublicURL != "" {
		config.publicUrl = access.PublicURL
	}
	if access.ConsoleURL != "" {
		config.consoleUrl = access.ConsoleURL
	}
	if specTrustProxy != nil {
		config.trustProxy = specTrustProxy
	} else if access.TrustProxy != nil {
		config.trustProxy = access.TrustProxy
	}
}
