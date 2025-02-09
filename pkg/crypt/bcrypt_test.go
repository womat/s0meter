package crypt

import "testing"

// TestBcrypt_Compare test the bcrypt hashing algorithm
func TestBcrypt_Compare(t *testing.T) {
	bc := NewBcrypt()
	hash, _ := bc.Encrypt("Vienna")
	bc.HashedText(hash)
	if bc.Compare() != true {
		t.Errorf("compare failed")
	}
}
