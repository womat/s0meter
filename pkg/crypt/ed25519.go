package crypt

import (
	"bytes"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/mikesmitty/edkey"
	"golang.org/x/crypto/ssh"
)

import (
	"golang.org/x/crypto/ed25519"
)

// GenerateEd25519KeyFiles generates two files
// id_ed25519 and id_ed25519.pub in directory dir.
// Source: https://github.com/mikesmitty/edkey/blob/master/edkey.go.
// It returns the fully qualified path to he public key file.
func GenerateEd25519KeyFiles(dir string, filename string) (string, error) {

	if dir == "" {
		dir = os.TempDir()
	}

	if filename == "" {
		filename = "id_ed25519"
	}

	var isFile = func(fileName string) bool {
		fi, err := os.Stat(fileName)
		return err == nil && !fi.IsDir()
	}

	privFileName := filepath.Join(filepath.ToSlash(dir), filename)
	pubFileName := privFileName + ".pub"

	if isFile(privFileName) || isFile(pubFileName) {
		return "", errors.New("file " + privFileName + " or " + pubFileName + " exists - do not overwrite")
	}

	// Generate a new private/public keypair for OpenSSH
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	publicKey, _ := ssh.NewPublicKey(pubKey)

	pemKey := &pem.Block{
		Type:  "OPENSSH PRIVATE KEY",
		Bytes: edkey.MarshalED25519PrivateKey(privKey),
	}
	privateKey := pem.EncodeToMemory(pemKey)
	authorizedKey := ssh.MarshalAuthorizedKey(publicKey)

	// append current timestamp at the end
	b := bytes.NewBuffer(authorizedKey[0 : len(authorizedKey)-1])
	b.WriteString(" " + time.Now().Format(time.RFC3339))
	b.WriteByte('\n')

	e1 := os.WriteFile(privFileName, privateKey, 0600)
	if e1 != nil {
		return "", e1
	}
	e2 := os.WriteFile(pubFileName, b.Bytes(), 0644)
	if e2 != nil {
		return "", e2
	}

	return pubFileName, nil
}
