package tls_util

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"golang.org/x/crypto/pkcs12"
	"io"
	"math/big"
	"net"
	"strings"
	"time"
)

// GenSelfSignedCerts generates a self-signed certificate and private key.
//   - cert: io.Writer certificate data is written to
//   - key: io.Writer private key data is written to
//   - host: the host name for the certificate
//   - isCA: whether the generated certificate is a CA certificate
//   - rsaBits: the number of bits for RSA keys
//   - ecdsaCurve: whether to use elliptic curve ECDSA keys (P224, P256, P384, P521) as private key, if empty RSA or Ed25519 keys are generated
//   - ed25519Key: whether to use Ed25519 keys as private key, if false RSA keys are generated
func GenSelfSignedCerts(cert, key io.Writer, host string, isCA bool, rsaBits int, ecdsaCurve string, ed25519Key bool) error {

	if host == "" {
		host = "localhost"
	}

	var publicKey = func(priv any) any {
		switch k := priv.(type) {
		case *rsa.PrivateKey:
			return &k.PublicKey
		case *ecdsa.PrivateKey:
			return &k.PublicKey
		case ed25519.PrivateKey:
			return k.Public().(ed25519.PublicKey)
		default:
			return nil
		}
	}

	var err error
	var priv any
	switch ecdsaCurve {
	case "":
		if ed25519Key {
			_, priv, err = ed25519.GenerateKey(rand.Reader)
		} else {
			priv, err = rsa.GenerateKey(rand.Reader, rsaBits)
		}
	case "P224":
		priv, err = ecdsa.GenerateKey(elliptic.P224(), rand.Reader)
	case "P256":
		priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "P384":
		priv, err = ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case "P521":
		priv, err = ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	default:
		err = fmt.Errorf("unrecognized elliptic curve '%q'", ecdsaCurve)
	}
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// ECDSA, ED25519 and RSA subject keys should have the DigitalSignature
	// KeyUsage bits set in the x509.Certificate template
	keyUsage := x509.KeyUsageDigitalSignature
	// Only RSA subject keys should have the KeyEncipherment KeyUsage bits set. In
	// the context of TLS this KeyUsage is particular to RSA key exchange and
	// authentication.
	if _, isRSA := priv.(*rsa.PrivateKey); isRSA {
		keyUsage |= x509.KeyUsageKeyEncipherment
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(15 * 365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   host,
			Organization: []string{"Acme Co"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	hosts := strings.Split(host, ",")
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, publicKey(priv), priv)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	if err := pem.Encode(cert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("encoding certificate failed: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("failed marshaling private key: %w", err)
	}

	if err := pem.Encode(key, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("encoding private key failed: %w", err)
	}

	return nil
}

// TLSConfigForPEM creates a tls.Config from a PEM encoded certificate and key.
//   - certPEM: the PEM encoded certificate
//
// - keyPEM: the PEM encoded private key
// Returns a tls.Config or an error if the certificate or key is invalid.
func TLSConfigForPEM(certPEM, keyPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pem certificate and key: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

// TLSConfigForPKCS12 creates a tls.Config from a PKCS12 encoded certificate.
// - data: the PKCS12 encoded certificate
// - password: the password for the PKCS12 encoded certificate
// Returns a tls.Config or an error if the certificate is invalid.
func TLSConfigForPKCS12(data []byte, password string) (*tls.Config, error) {
	cert, err := LoadPKCS12(data, password)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}

// LoadPKCS12 loads a PKCS12 encoded certificate.
// - data: the PKCS12 encoded certificate
// - password: the password for the PKCS12 encoded certificate
// Returns a tls.Certificate or an error if the certificate is invalid.
func LoadPKCS12(data []byte, password string) (tls.Certificate, error) {
	key, cert, err := pkcs12.Decode(data, password)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed decoding pkcs12 certificate: %w", err)
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  key.(crypto.PrivateKey),
		Leaf:        cert,
	}

	return tlsCert, nil
}
