package v3

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestControlPlaneSpec_UnmarshalRefSample(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)

	data, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "testdata", "controlplane-ref.yaml"))
	require.NoError(t, err)

	var cp ControlPlane
	require.NoError(t, yaml.Unmarshal(data, &cp))

	require.Equal(t, "iofog", cp.Name)
	require.Equal(t, AuthModeEmbedded, cp.Spec.Auth.Mode)
	require.NotNil(t, cp.Spec.Auth.Bootstrap)
	require.Equal(t, "admin", cp.Spec.Auth.Bootstrap.Username)
	require.Equal(t, "ReplaceMe1!", cp.Spec.Auth.Bootstrap.Password)
	require.Equal(t, "https://controller.example.com", cp.Spec.Controller.PublicUrl)
	require.NotNil(t, cp.Spec.Controller.TrustProxy)
	require.True(t, *cp.Spec.Controller.TrustProxy)
	require.Equal(t, int32(2), cp.Spec.Replicas.Controller)
	require.Equal(t, int32(2), cp.Spec.Replicas.Nats)
}
