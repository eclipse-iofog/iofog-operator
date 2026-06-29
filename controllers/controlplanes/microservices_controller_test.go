package controllers

import (
	"fmt"
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

func TestNewControllerMicroservice_ExposesAPIAndConsoleServicePorts(t *testing.T) {
	ms := newControllerMicroservice("cp-ns", &controllerMicroserviceConfig{
		replicas: 1,
		db:       &cpv3.Database{Provider: "postgres"},
		auth:     &cpv3.Auth{Mode: cpv3.AuthModeEmbedded},
	})

	require.Len(t, ms.services, 1)
	require.Len(t, ms.services[0].ports, 2)

	require.Equal(t, "controller-api", ms.services[0].ports[0].Name)
	require.Equal(t, int32(controllerAPIPort), ms.services[0].ports[0].Port)
	require.Equal(t, int32(controllerAPIPort), ms.services[0].ports[0].TargetPort.IntVal)

	require.Equal(t, controllerConsolePortName, ms.services[0].ports[1].Name)
	require.Equal(t, int32(controllerConsoleServicePort), ms.services[0].ports[1].Port)
	require.Equal(t, int32(defaultControllerConsolePort), ms.services[0].ports[1].TargetPort.IntVal)
}

func TestNewControllerMicroservice_ConsolePortFromCR(t *testing.T) {
	ms := newControllerMicroservice("cp-ns", &controllerMicroserviceConfig{
		replicas:    1,
		consolePort: 9000,
		db:          &cpv3.Database{Provider: "postgres"},
		auth:        &cpv3.Auth{Mode: cpv3.AuthModeEmbedded},
	})

	require.Equal(t, int32(9000), ms.services[0].ports[1].TargetPort.IntVal)

	var consoleEnv string
	for _, e := range ms.containers[0].env {
		if e.Name == "CONSOLE_PORT" {
			consoleEnv = e.Value
			break
		}
	}
	require.Equal(t, "9000", consoleEnv)
}

func TestControllerReadinessProbe_HTTPUsesHTTPGet(t *testing.T) {
	probe := controllerReadinessProbe(&controllerMicroserviceConfig{})

	require.NotNil(t, probe.HTTPGet)
	require.Nil(t, probe.Exec)
	require.Equal(t, "/api/v3/status", probe.HTTPGet.Path)
	require.Equal(t, corev1.URISchemeHTTP, probe.HTTPGet.Scheme)
	require.Equal(t, int32(controllerAPIPort), probe.HTTPGet.Port.IntVal)
}

func TestControllerReadinessProbe_HTTPSUsesCurlExec(t *testing.T) {
	probe := controllerReadinessProbe(&controllerMicroserviceConfig{https: ptr.To(true)})

	require.NotNil(t, probe.Exec)
	require.Nil(t, probe.HTTPGet)
	require.Equal(t, []string{
		"curl",
		"-sfk",
		fmt.Sprintf("https://127.0.0.1:%d/api/v3/status", controllerAPIPort),
	}, probe.Exec.Command)
}

func TestNewControllerMicroservice_ReadinessProbeMatchesHTTPSConfig(t *testing.T) {
	base := &controllerMicroserviceConfig{
		replicas: 1,
		db:       &cpv3.Database{Provider: "postgres"},
		auth:     &cpv3.Auth{Mode: cpv3.AuthModeEmbedded},
	}

	msHTTP := newControllerMicroservice("cp-ns", base)
	require.NotNil(t, msHTTP.containers[0].readinessProbe.HTTPGet)
	require.Nil(t, msHTTP.containers[0].readinessProbe.Exec)

	cfgHTTPS := *base
	cfgHTTPS.https = ptr.To(true)
	msHTTPS := newControllerMicroservice("cp-ns", &cfgHTTPS)
	require.NotNil(t, msHTTPS.containers[0].readinessProbe.Exec)
	require.Nil(t, msHTTPS.containers[0].readinessProbe.HTTPGet)
}

func TestNewControllerMicroservice_ConsoleURLFromCR(t *testing.T) {
	ms := newControllerMicroservice("cp-ns", &controllerMicroserviceConfig{
		replicas:   1,
		consoleUrl: "https://ui.example.com",
		db:         &cpv3.Database{Provider: "postgres"},
		auth:       &cpv3.Auth{Mode: cpv3.AuthModeEmbedded},
	})

	var consoleURL string
	for _, e := range ms.containers[0].env {
		if e.Name == "CONSOLE_URL" {
			consoleURL = e.Value
			break
		}
	}
	require.Equal(t, "https://ui.example.com", consoleURL)
}
