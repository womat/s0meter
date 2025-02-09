package crypt

import (
	"testing"
)

var (
	keyAndValue = map[string]string{
		"":  "Hello World",
		"x": "Hello World",
		"lsjfdhlakjdshflkajslfkjaljahlkjdshflkajsljsalaf": "x",
		"12345678901234567890123456789011":                "aaaaaaaaaaa",
	}
)

// TestAESExample is the easiest way to encrypt and decrypt
func TestAESExample(t *testing.T) {
	cipher := NewSymmetricEncryption().SetPlainText("mySecretPassword").GetCypherBase64()
	// cipher e.g. "gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M="
	plain, _ := NewSymmetricEncryption().SetCypherBase64(cipher).GetPlainText()
	if plain != "mySecretPassword" {
		t.Errorf("Expected 'mySecretPassword'")
	}
}

// TestAESExampleTwice decrypts twice
func TestAESExampleTwice(t *testing.T) {
	cipher := NewSymmetricEncryption().SetPlainText("mySecretPassword").GetCypherBase64()
	// cipher e.g. "gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M="
	s := NewSymmetricEncryption().SetCypherBase64(cipher)
	plain, _ := s.GetPlainText()
	if plain != "mySecretPassword" {
		t.Errorf("Expected 'mySecretPassword'")
	}
}

// TestAESCipher tests setting a cipher and give it back in the next step
func TestAESCipher(t *testing.T) {
	cipher := NewSymmetricEncryption().SetCypherBase64("gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M=")
	back := cipher.GetCypherBase64()
	if back != "gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M=" {
		t.Errorf("Expected 'gQTf9p9trkvx3xvSKAQhLIfqUWirxnf5Sl+YzAKV5zjS0hLZ0LRXs+1rb6M='")
	}
}

// TestAESEncryptionForward loops over keyAndValue.
func TestAESEncryptionForward(t *testing.T) {
	var (
		err               error
		plainBack, cypher string
	)

	for key, plaintext := range keyAndValue {

		s := NewSymmetricEncryption().SetKey(key).SetPlainText(plaintext)

		cypher = s.GetCypherBase64()
		if cypher == "" {
			t.Errorf("GetCypherBase64 with key %s and value %s returned an error %s", key, plaintext, err)
		}

		s.SetCypherBase64(cypher)

		plainBack, _ = s.GetPlainText()
		if plainBack != plaintext {
			t.Errorf("Encryption with key %s and value %s returned false result %s", key, plaintext, plainBack)
		}
	}
}

// TestAESEncryptionReverse takes keyAndValue map in opposite direction.
func TestAESEncryptionReverse(t *testing.T) {
	var (
		err               error
		plainBack, cypher string
	)

	for plaintext, key := range keyAndValue {

		s := NewSymmetricEncryption()
		s.SetKey(key).SetPlainText(plaintext)

		cypher = s.GetCypherBase64()
		if cypher == "" {
			t.Errorf("GetCypherBase64 with key %s and value %s returned an error %s", key, plaintext, err)
		}

		s.SetCypherBase64(cypher)

		plainBack, _ = s.GetPlainText()
		if plainBack != plaintext {
			t.Errorf("Encryption with key %s and value %s returned false result %s", key, plaintext, plainBack)
		}
	}
}

// TestAESEncryptionSpecial tests empty key and values.
func TestAESEncryptionSpecial(t *testing.T) {
	var (
		err               error
		plainBack, cypher string
	)

	s := NewSymmetricEncryption().SetPlainText("")

	cypher = s.GetCypherBase64()
	if cypher == "" {
		t.Errorf("GetCypherBase64 with empty key and value returned an error %s", err)
	}

	s.SetCypherBase64(cypher)
	plainBack, _ = s.GetPlainText()
	if plainBack != "" {
		t.Errorf("Encryption with empty key and value returned false result %s", plainBack)
	}

	s = NewSymmetricEncryption().SetKey("").SetPlainText("")

	cypher = s.GetCypherBase64()
	if cypher == "" {
		t.Errorf("GetCypherBase64 with empty key and value returned an error %s", err)
	}

	s.SetCypherBase64(cypher)
	plainBack, _ = s.GetPlainText()
	if plainBack != "" {
		t.Errorf("Encryption with empty key and value returned false result %s", plainBack)
	}
}

// TestAESPlainBack tests empty key and values.
func TestAESPlainBack(t *testing.T) {
	s := NewSymmetricEncryption().SetPlainText("x")
	if s.GetCypherBase64() == "" {
		t.Errorf("Expected a valid cypher and got ''")
	}
}

// TestAESError bad cypher text
func TestAESError(t *testing.T) {
	s := NewSymmetricEncryption().SetCypherBase64("x")
	pt, err := s.GetPlainText()
	if pt == "" && err != nil {
		return
	}
	t.Errorf("Expected an error")
}
