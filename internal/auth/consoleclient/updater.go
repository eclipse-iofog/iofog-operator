package consoleclient

import (
	"context"

	cpv3 "github.com/eclipse-iofog/iofog-operator/v3/apis/controlplanes/v3"
	"github.com/go-logr/logr"
)

// Config holds auth fields needed to update console OIDC client redirect URLs.
type Config struct {
	Mode                 cpv3.AuthMode
	IssuerURL            string
	ConsoleClient        string
	ConsoleClientEnabled bool
}

// Updater updates OIDC console client redirect URLs at the IdP.
type Updater interface {
	UpdateConsoleURLs(ctx context.Context, cfg Config, consoleURL string) error
}

// NoopUpdater is the default v3.8 stub - logs at V(1) and returns nil.
type NoopUpdater struct {
	Log logr.Logger
}

func (u *NoopUpdater) UpdateConsoleURLs(_ context.Context, cfg Config, consoleURL string) error {
	u.Log.V(1).Info("console client URL update skipped (noop updater)",
		"consoleURL", consoleURL,
		"mode", cfg.Mode,
		"consoleClient", cfg.ConsoleClient,
		"consoleClientEnabled", cfg.ConsoleClientEnabled,
	)
	return nil
}

// ConfigFromAuth builds updater config from CR auth spec.
func ConfigFromAuth(auth cpv3.Auth) Config {
	cfg := Config{
		Mode:          auth.Mode,
		IssuerURL:     auth.IssuerUrl,
		ConsoleClient: auth.ConsoleClient,
	}
	if auth.ConsoleClientEnabled != nil {
		cfg.ConsoleClientEnabled = *auth.ConsoleClientEnabled
	}
	return cfg
}
