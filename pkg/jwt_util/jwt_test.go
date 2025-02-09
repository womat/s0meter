package jwt_util

import (
	"testing"
	"time"
)

func TestGenerateAndValidateToken(t *testing.T) {
	t.Parallel()

	t.Run("valid token", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "issuer", "auth", "id", "secret")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		if claims.User != "user" {
			t.Errorf("expected user, got %s", claims.User)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token+"invalid", "issuer", "auth", "id", "secret")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

	t.Run("invalid issuer", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "invalid", "auth", "id", "secret")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

	t.Run("invalid subject", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "issuer", "invalid", "id", "secret")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "issuer", "auth", "invalid", "secret")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

	t.Run("invalid secret", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", 5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "issuer", "auth", "id", "invalid")
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		t.Parallel()

		token, err := GenerateToken("user", "issuer", "auth", "id", "secret", -5*time.Minute)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
		claims, err := ValidateToken(token, "issuer", "auth", "id", "secret")
		t.Log(err)
		if err == nil {
			t.Error("expected error, got nil")
		}
		if claims != nil {
			t.Errorf("expected nil, got %v", claims)
		}
	})

}
