package app

import (
	"crypto/tls"
	_ "embed"
	"errors"
	"log/slog"
	"net/http"
	"s0counter/pkg/tls_util"
	"s0counter/pkg/web"
)

// generate a self-signed certificate for development
// openssl req -x509 -nodes -newkey rsa:2048 -keyout selfsigned.key -out selfsigned.crt -days 35600 -subj "/C=AT/ST=Vienna/L=Vienna/O=ITDesign/OU=DEV/CN=localhost/emailAddress=support@itdesign.at"
// -subj description
// /C=AT								Country
// /ST=Vienna							State (optional).
// /L=Vienna							Location – City (optional).
// /O=company							company (optional).
// /OU=IT								Organizational Unit – (optional).
// /CN=my-domain.com					Common Name – IMPORTANT! your domain name or localhost.
// /emailAddress=admin@my-domain.com	E-Mail-Address (optional).

//go:embed cert.pem
var cert []byte

//go:embed key.pem
var key []byte

// StartWebServer initializes and starts the web server in a separate Goroutine.
// It configures TLS based on the environment:
// - In development mode, embedded self-signed certificates are used.
// - In production, certificates are loaded from the configured files, with a fallback to embedded certificates if necessary.
//
// The function does not block execution. Errors occurring during setup are returned immediately.
// Runtime errors (e.g., failure in Serve()) are logged but do not propagate.
//
// Returns an error if the server cannot be initialized.
func (app *App) StartWebServer() error {
	var tlsConfig *tls.Config
	var err error

	if app.config.IsDevEnv() {
		slog.Info("Using embedded certificates for development")
		// use embedded certificates for development
		if tlsConfig, err = tls_util.TLSConfigForPEM(cert, key); err != nil {
			slog.Error("Failed to create tls config from embedded certificates", "error", err)
			return err
		}
	} else {
		if tlsConfig, err = web.SetTLS(app.config.Webserver.CertFile, app.config.Webserver.KeyFile, app.config.Webserver.CertPassword.Value()); err != nil {
			slog.Warn("Failed to create tls config from given certificates, using embedded certificates", "error", err)
			if tlsConfig, err = tls_util.TLSConfigForPEM(cert, key); err != nil {
				slog.Error("Failed to create tls config from embedded certificates", "error", err)
				return err
			}
		}
	}

	web.SetTLSMinVersion(tlsConfig, app.config.Webserver.MinTLS)
	listener, err := web.NewListener(tlsConfig, app.config.Webserver.ListenHost, app.config.Webserver.ListenPort)
	if err != nil {
		slog.Error("Failed to create listener", "error", err)
		return err
	}

	go func() {
		slog.Info("Starting webserver", "host", app.config.Webserver.ListenHost, "port", app.config.Webserver.ListenPort)
		if err := app.web.Serve(listener.Listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed serving", "error", err)
		}

		if err := listener.Close(); err != nil {
			slog.Error("Failed to close listener", "error", err)
		}
	}()

	return nil
}
