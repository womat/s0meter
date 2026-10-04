package app

import (
	"path/filepath"
	"testing"
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
