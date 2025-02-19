package app

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
)

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

	listener, err := net.Listen("tcp4", app.config.HttpsServer.ListenHost+":"+app.config.HttpsServer.ListenPort)
	if err != nil {
		slog.Error("Failed to create listener", "error", err)
		return err
	}

	go func() {
		slog.Info("Starting webserver", "host", app.config.HttpsServer.ListenHost, "port", app.config.HttpsServer.ListenPort)
		if err := app.web.ServeTLS(listener, app.config.HttpsServer.CertFile, app.config.HttpsServer.KeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed serving", "error", err)
		}

		if err := listener.Close(); err != nil {
			slog.Error("Failed to close listener", "error", err)
		}
	}()

	return nil
}
