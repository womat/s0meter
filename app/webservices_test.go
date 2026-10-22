package app

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
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

// A client that offers at most TLS 1.2 is refused.
func TestTLSConfigRequires13(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pem")
	cert, err := loadTLSCert(missing, missing, DevEnv)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.TLS = newTLSConfig(cert)
	srv.StartTLS()
	defer srv.Close()

	for _, tt := range []struct {
		max  uint16
		fail bool
	}{
		{tls.VersionTLS12, true},
		{tls.VersionTLS13, false},
	} {
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // self-signed test certificate
			MaxVersion:         tt.max,
		}}}
		resp, err := client.Get(srv.URL)
		if err == nil {
			_ = resp.Body.Close()
		}
		if (err != nil) != tt.fail {
			t.Errorf("max version %x: err = %v, want failure %v", tt.max, err, tt.fail)
		}
	}
}

// The real server of a started App speaks TLS 1.3 only, and a stopped App releases its port
// for the next one.
func TestRunServesTLS13AndFreesThePort(t *testing.T) {
	dir := t.TempDir()
	cfg := NewConfig()
	cfg.DataFile = filepath.Join(dir, "s0meter.yaml")
	cfg.Webserver.ListenHost = "127.0.0.1"
	cfg.Webserver.ListenPort = 0 // StartWebServer records the port it got in Addr
	cfg.Webserver.CertFile = filepath.Join(dir, "missing.pem")
	cfg.Webserver.KeyFile = cfg.Webserver.CertFile
	cfg.Env = DevEnv

	signals := make(chan os.Signal, 1)
	app, err := New(cfg, signals, nil).Run()
	if err != nil {
		t.Fatal(err)
	}
	addr := app.web.Addr

	for _, tt := range []struct {
		max  uint16
		fail bool
	}{
		{tls.VersionTLS12, true},
		{tls.VersionTLS13, false},
	} {
		client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // embedded development certificate
			MaxVersion:         tt.max,
		}}}
		resp, err := client.Get("https://" + addr + "/version")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("GET /version = %d, want 200", resp.StatusCode)
			}
		}
		if (err != nil) != tt.fail {
			t.Errorf("max version %x: err = %v, want failure %v", tt.max, err, tt.fail)
		}
	}

	signals <- syscall.SIGTERM
	select {
	case <-app.Shutdown():
	case <-time.After(10 * time.Second):
		t.Fatal("SIGTERM did not stop the App")
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("port still taken after the App stopped: %v", err)
	}
	_ = l.Close()
}
