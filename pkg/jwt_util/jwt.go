package jwt_util

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

var (
	ErrInvalidToken   = errors.New("token is invalid")
	ErrExpiredToken   = errors.New("token has expired")
	ErrInvalidIssuer  = errors.New("token has invalid issuer")
	ErrInvalidSubject = errors.New("token has invalid subject")
	ErrInvalidID      = errors.New("token has invalid ID")
)

// Claims represents the JWT token claims.
type Claims struct {
	User string `json:"user"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for a user.
//
//	username: the user's name
//	issuer: the token issuer, e.g. the application name
//	subject: the token subject, e.g. auth
//	id: unique app token id to prevent token reuse
//	secret: the secret used to sign the token
//	lifetime: the token lifetime
//
// Returns the token string or an error if the token could not be generated.
func GenerateToken(user, issuer, subject, id, secret string, lifetime time.Duration) (string, error) {
	expiresAt := jwt.NewNumericDate(time.Now().Add(lifetime))
	now := time.Now()
	claims := &Claims{
		User: user,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   subject,
			ExpiresAt: expiresAt,
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        id,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken validates the JWT token.
//
//	tokenString: the token string
//	issuer: the token issuer, e.g. the application name
//	subject: the token subject, e.g. auth
//	id: unique app token id
//	secret: the secret used to sign the token
//
// Returns the token claims or an error if the token is invalid.
func ValidateToken(tokenString string, issuer, subject, id, secret string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})

	// Check if token is invalid
	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrInvalidToken
		}
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Validate issuer
	if claims.Issuer != issuer {
		return nil, ErrInvalidIssuer
	}

	// Validate subject
	if claims.Subject != subject {
		return nil, ErrInvalidSubject
	}

	// Validate ID
	if claims.ID != id {
		return nil, ErrInvalidID
	}

	return claims, nil
}
