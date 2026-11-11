package observability

import (
	"log/slog"
	"time"

	connect "connectrpc.com/connect"
	"github.com/newrelic/go-agent/v3/integrations/nrconnect"
	"github.com/newrelic/go-agent/v3/integrations/nrslog"
	"github.com/newrelic/go-agent/v3/newrelic"
)

// Config holds configuration parameters for New Relic telemetry integration.
type Config struct {
	AppName    string
	LicenseKey string
}

// InitNewRelic initializes and connects to New Relic if a license key is provided in Config.
// If LicenseKey is empty, it safely returns nil, nil without initializing New Relic.
func InitNewRelic(cfg Config, logger *slog.Logger) (*newrelic.Application, error) {
	if cfg.LicenseKey == "" {
		return nil, nil
	}

	opts := []newrelic.ConfigOption{
		newrelic.ConfigAppName(cfg.AppName),
		newrelic.ConfigLicense(cfg.LicenseKey),
	}
	if logger != nil {
		opts = append(opts, nrslog.ConfigLogger(logger.WithGroup("newrelic")))
	}

	app, err := newrelic.NewApplication(opts...)
	if err != nil {
		return nil, err
	}
	if waitErr := app.WaitForConnection(5 * time.Second); waitErr != nil {
		if logger != nil {
			logger.Warn("newrelic.connect_failed", "error", waitErr.Error())
		}
	}
	return app, nil
}

// ConnectHandlerOptions returns ConnectRPC handler options that instrument RPC endpoints
// with New Relic transaction tracing and error reporting.
func ConnectHandlerOptions(app *newrelic.Application) []connect.HandlerOption {
	if app == nil {
		return nil
	}
	return []connect.HandlerOption{connect.WithInterceptors(nrconnect.Interceptor(app))}
}

// ConnectClientOptions returns ConnectRPC client options that instrument outgoing RPC calls
// with New Relic external segment tracing.
func ConnectClientOptions(app *newrelic.Application) []connect.ClientOption {
	if app == nil {
		return nil
	}
	return []connect.ClientOption{connect.WithInterceptors(nrconnect.Interceptor(app))}
}
