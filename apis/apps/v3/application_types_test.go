package v3

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestApplicationSpec_UnmarshalRefSample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "testdata", "application-ref.yaml"))
	require.NoError(t, err)

	var app Application
	require.NoError(t, yaml.Unmarshal(data, &app))

	require.Equal(t, "health-care-wearable", app.Name)
	require.NotNil(t, app.Spec.NatsConfig)
	require.True(t, app.Spec.NatsConfig.NatsAccess)
	require.Equal(t, "default-account", app.Spec.NatsConfig.NatsRule)
	require.Len(t, app.Spec.Microservices, 2)

	monitor := app.Spec.Microservices[0]
	require.Equal(t, "heart-rate-monitor", monitor.Name)
	require.Equal(t, "horse-1", monitor.Agent.Name)
	require.Equal(t, 50, monitor.Schedule)
	require.NotNil(t, monitor.Images)
	require.Equal(t, "arm-image:latest", monitor.Images.AMD64)
	require.NotNil(t, monitor.NatsConfig)
	require.True(t, monitor.NatsConfig.NatsAccess)
	require.Equal(t, "default-user", monitor.NatsConfig.NatsRule)
	require.NotNil(t, monitor.Container.MemoryLimit)
	require.Equal(t, int64(4096), *monitor.Container.MemoryLimit)
	require.NotNil(t, monitor.Container.HealthCheck)
	require.Len(t, monitor.Container.HealthCheck.Test, 2)

	viewer := app.Spec.Microservices[1]
	require.Equal(t, "heart-rate-viewer", viewer.Name)
	require.Equal(t, "tcp", viewer.Container.Ports[0].Protocol)
}

func TestApplicationSpec_UnmarshalTemplateRefSample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "testdata", "application-template-ref.yaml"))
	require.NoError(t, err)

	var app Application
	require.NoError(t, yaml.Unmarshal(data, &app))

	require.NotNil(t, app.Spec.Template)
	require.Equal(t, "template-name", app.Spec.Template.Name)
	require.Len(t, app.Spec.Template.Variables, 1)
	require.Equal(t, "variable-name", app.Spec.Template.Variables[0].Key)
	require.NotNil(t, app.Spec.Template.Variables[0].Value)
}
