package consoleclient

import (
	"context"
	"testing"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestNoopUpdater_UpdateConsoleURLs(t *testing.T) {
	u := &NoopUpdater{Log: logr.Discard()}
	err := u.UpdateConsoleURLs(context.Background(), Config{
		Mode:                 cpv3.AuthModeEmbedded,
		ConsoleClient:        "edgeops-console",
		ConsoleClientEnabled: true,
	}, "https://console.example.com")
	require.NoError(t, err)
}

func TestConfigFromAuth(t *testing.T) {
	cfg := ConfigFromAuth(cpv3.Auth{
		Mode:                 cpv3.AuthModeExternal,
		IssuerUrl:            "https://idp.example.com/realms/pot",
		ConsoleClient:        "edgeops-console",
		ConsoleClientEnabled: ptr.To(true),
	})
	require.Equal(t, cpv3.AuthModeExternal, cfg.Mode)
	require.Equal(t, "https://idp.example.com/realms/pot", cfg.IssuerURL)
	require.Equal(t, "edgeops-console", cfg.ConsoleClient)
	require.True(t, cfg.ConsoleClientEnabled)
}
