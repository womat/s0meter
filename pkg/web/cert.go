package web

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"s0counter/pkg/tls_util"
	"strings"
)

var ErrCertFileExists = errors.New("certificate file already exists")

// SetTLS creates a tls.Config object from the given certificate and key files.
func SetTLS(certFilePath, keyFilePath, certPassword string) (*tls.Config, error) {
	var tlsConfig *tls.Config

	cert, err := os.ReadFile(certFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading certificate file %v: %w", certFilePath, err)
	}

	// check for pfx certificate file.
	if strings.HasSuffix(certFilePath, ".pfx") {
		tlsConfig, err = tls_util.TLSConfigForPKCS12(cert, certPassword)
		return tlsConfig, err
	}

	// it's a pem certificate files.
	key, err := os.ReadFile(keyFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed reading certificate file %v: %w", keyFilePath, err)
	}

	tlsConfig, err = tls_util.TLSConfigForPEM(cert, key)
	return tlsConfig, err
}

// GenSelfSignedCerts generates self-signed certificates if not provided.
func GenSelfSignedCerts(certFilePath, keyFilePath string) error {

	if fileInfo, err := os.Stat(certFilePath); err == nil && fileInfo.Size() > 0 {
		return ErrCertFileExists
	}

	var cert, key bytes.Buffer
	if err := tls_util.GenSelfSignedCerts(&cert, &key, "localhost", false, 2048, "", false); err != nil {
		return err
	}

	if err := os.WriteFile(certFilePath, cert.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed writing certificate file %v: %w", certFilePath, err)
	}

	if err := os.WriteFile(keyFilePath, key.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed writing key file %v: %w", keyFilePath, err)
	}

	return nil
}

// SetTLSMinVersion checks the minimum TLS version and sets the ciphers accordingly.
func SetTLSMinVersion(tlsConfig *tls.Config, minTLS string) {

	// check config parameter like "url: https://0.0.0.0:7844/?minTlsVersion=1.2"
	// which allows to overwrite the expected minimum TLS version

	// use TLS 1.3 as default (be as secure as possible)
	minTlsVersion := uint16(tls.VersionTLS13)

	var ciphers []uint16

	switch minTLS {
	case "1.0": // should not be used
		slog.Debug("Using TLS 1.0")
		minTlsVersion = tls.VersionTLS10
	case "1.1": // should not be used
		slog.Debug("Using TLS 1.1")
		minTlsVersion = tls.VersionTLS11
	case "1.2a": // a ... use all ciphers from golang (even insecure one)
		slog.Debug("Using TLS 1.2")
		minTlsVersion = tls.VersionTLS12
	case "1.2": // from: https://ssl-config.mozilla.org/#server=go&version=1.14.4&config=intermediate&guideline=5.6
		slog.Debug("Using TLS 1.2 with secure ciphers")
		minTlsVersion = tls.VersionTLS12
		ciphers = []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		}
	}

	tlsConfig.MinVersion = minTlsVersion
	if len(ciphers) > 0 {
		tlsConfig.CipherSuites = ciphers
	}
}
