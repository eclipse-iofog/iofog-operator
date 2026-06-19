package controllers

import (
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/stretchr/testify/require"
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
