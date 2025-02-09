package crypt

import (
	"golang.org/x/crypto/bcrypt"
)

// Bcrypt is the main interface
type Bcrypt interface {
	// Encrypt takes plainText and encrypts it with bcrypt and its costs (default = 4).
	// It returns the encrypted string or an empty string on error.
	Encrypt(plainText string) (string, error)

	// Cost sets bcrypt costs (and checks its bounds)
	Cost(cost int)

	// HashedText allows to set a hased text string like e.g. $2a$04$vH2ABneGM7Lpl0oGkvAgtOCIF29Ku.TUPx5.CLD1.EKLl2RgT6ilK
	HashedText(hashedText string)

	// PlainText allows to set a plain text string
	PlainText(plainText string)

	// Compare checks the hash against the plaintext (hashed) and returns true
	// on success
	Compare() bool
}

type bcryptProperties struct {
	plainText  string
	hashedText string
	cost       int
}

// NewBcrypt is the main entry point
func NewBcrypt() Bcrypt {
	return &bcryptProperties{
		cost: 4,
	}
}

// Encrypt takes plainText and encrypts it with bcrypt and its costs (default = 4).
// It returns the encrypted string or an empty string on error.
func (b *bcryptProperties) Encrypt(plainText string) (string, error) {
	var err error
	var x []byte

	b.plainText = plainText

	x, err = bcrypt.GenerateFromPassword([]byte(b.plainText), b.cost)
	if err != nil {
		return "", err
	}

	b.hashedText = string(x)
	return b.hashedText, nil
}

// HashedText allows to set a hased text string like e.g.
func (b *bcryptProperties) HashedText(hashedText string) {
	b.hashedText = hashedText
}

// PlainText allows to set a plain text string
func (b *bcryptProperties) PlainText(plainText string) {
	b.plainText = plainText
}

// Cost sets bcrypt costs (and checks its bounds)
func (b *bcryptProperties) Cost(cost int) {

	b.cost = cost

	if b.cost < bcrypt.MinCost {
		b.cost = bcrypt.MinCost
		return
	}
	if b.cost > bcrypt.MaxCost {
		b.cost = bcrypt.MaxCost
	}
}

// Compare checks the hash against the plaintext (hashed) and returns true
// on success.
func (b *bcryptProperties) Compare() bool {
	err := bcrypt.CompareHashAndPassword([]byte(b.hashedText), []byte(b.plainText))
	return err == nil
}
