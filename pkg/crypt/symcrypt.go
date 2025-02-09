package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Do not change this constant. Use your own key in production
// environment.
const defaultSymmetricKey = "42aaee02Qb687X4d7cA9521T82464264"

// AES key length - do not change
const keyLen = 32

// SymCrypt is the main structure for symmetric encryption and holds
// plainText, key and cipher as unexported fields.
//
// Usage sample:
//
//	// cipher e.g. "gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M="
//	cipher := crypt.NewSymmetricEncryption().SetPlainText("mySecretPassword").GetCypherBase64()
//	plain := crypt.NewSymmetricEncryption().SetCypherBase64(cipher).GetPlainText()
//	if plain != "mySecretPassword" {
//		log.Println("ERROR: expected 'mySecretPassword'")
//	}
type SymCrypt struct {
	// readable text
	plainText string

	// 32 char - key
	key string

	// scrambled (encrypted) byte slice of plainText
	cipher []byte

	// flag to see it the plaintext is already encrypted
	flag int
}

var (
	errCipherTextTooShort = errors.New("cipher text too short")
)

const (
	reset         = iota
	hasPlainText  = 1
	hasCipherText = 2
	hasEncrypted  = 4
	hasDecrypted  = 8
)

// NewSymmetricEncryption is the entry point for AES encryption/decryption and
// can be used to encrypt a string and get the result as base64 string.
// When the key is not configured it uses a built in one (should not be used
// in production environment).
func NewSymmetricEncryption() *SymCrypt {
	return &SymCrypt{
		key:  defaultSymmetricKey,
		flag: reset,
	}
}

// SetKey adds an AES key - the length is not so important because
// it is stripped or expanded to exactly 32 chars (256 bit).
func (s *SymCrypt) SetKey(key string) *SymCrypt {
	l := len(key)

	if l != keyLen {
		if l < keyLen {
			s.key = key + defaultSymmetricKey[0:keyLen-len(key)]
		} else {
			s.key = key[0:keyLen]
		}
	} else {
		s.key = key
	}

	return s
}

// SetPlainText text adds the text we want to encrypt.
func (s *SymCrypt) SetPlainText(plainText string) *SymCrypt {
	s.plainText = plainText
	s.flag = hasPlainText
	return s
}

// GetCypherBase64 returns the encrypted data stream as base64 encoded
// string like e.g. 8q+orlJS5rzn0HtzbmFIkJIGAoOIL3zczlXVTUylRU021g==.
// It returns an empty string on error
func (s *SymCrypt) GetCypherBase64() string {
	if len(s.cipher) < 1 {
		_ = s.encrypt()
	}

	var ret = base64.StdEncoding.EncodeToString(s.cipher)

	return ret
}

// SetCypherBase64 adds s.cipher - but as base64 string.
func (s *SymCrypt) SetCypherBase64(base64String string) *SymCrypt {
	var (
		b   []byte
		err error
	)

	b, err = base64.StdEncoding.DecodeString(base64String)
	if err == nil {
		s.flag = hasCipherText
		s.cipher = b
	}

	return s
}

// GetPlainText returns the plaintext
func (s *SymCrypt) GetPlainText() (string, error) {
	err := s.decrypt()
	return s.plainText, err
}

// encrypt (Verschluesseln) takes s.plainText and encrypts it
// the result is stored into s.cipher as byte slice.
func (s *SymCrypt) encrypt() error {
	if s.flag&hasPlainText == hasPlainText && s.flag&hasEncrypted == hasEncrypted {
		return nil
	}
	var err error
	if s.flag&hasPlainText == hasPlainText {
		s.cipher, err = s.byteEncrypt()
		if err == nil {
			s.flag = s.flag | hasEncrypted
		}
	}
	return err
}

// Decrypt decrypts an authenticated cipher stored in s.cypher and stores
// it into s.plainText.
func (s *SymCrypt) decrypt() error {
	if s.flag&hasCipherText == hasCipherText && s.flag&hasDecrypted == hasDecrypted {
		return nil
	}
	var err error
	var b []byte
	if s.flag&hasCipherText == hasCipherText {
		b, err = s.byteDecrypt()
		if err == nil {
			s.plainText = string(b)
			s.flag = s.flag | hasCipherText
			s.flag = s.flag | hasPlainText
			s.flag = s.flag | hasEncrypted
			s.flag = s.flag | hasDecrypted
		}
	} else {
		return errors.New("no cipher text")
	}
	return err
}

// byteEncrypt encrypts and authenticates plaintext.
func (s *SymCrypt) byteEncrypt() (b []byte, err error) {
	var (
		c   cipher.Block
		gcm cipher.AEAD
	)

	c, err = aes.NewCipher([]byte(s.key))
	if err != nil {
		return nil, err
	}

	gcm, err = cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, []byte(s.plainText), nil), nil
}

// byteDecrypt decrypts and authenticates ciphertext.
func (s *SymCrypt) byteDecrypt() (b []byte, err error) {
	var (
		c   cipher.Block
		gcm cipher.AEAD
	)

	c, err = aes.NewCipher([]byte(s.key))
	if err != nil {
		return
	}

	gcm, err = cipher.NewGCM(c)
	if err != nil {
		return
	}

	nonceSize := gcm.NonceSize()
	if len(s.cipher) < nonceSize {
		return nil, errCipherTextTooShort
	}

	nonce, ciphertext := s.cipher[:nonceSize], s.cipher[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
