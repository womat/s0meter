package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadTLSCertFallback(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pem")

	if _, err := loadTLSCert(missing, missing, ProdEnv); err == nil {
		t.Error("env prod without certFile must fail instead of using the embedded certificate")
	}
	if _, err := loadTLSCert(missing, missing, DevEnv); err != nil {
		t.Errorf("env dev should fall back to the embedded certificate: %v", err)
	}
}

func TestServerErrorRestartsApp(t *testing.T) {
	cfg := NewConfig()
	cfg.DataFile = filepath.Join(t.TempDir(), "s0meter.yaml")
	app := New(cfg, make(chan os.Signal), nil)
	app.HandleOSSignals()

	app.serverErr <- errors.New("listener died")

	select {
	case <-app.Restart():
	case <-time.After(5 * time.Second):
		t.Fatal("a web server error did not restart the App")
	}
	if app.ctx.Err() == nil {
		t.Error("the App context was not cancelled by the restart")
	}
}
