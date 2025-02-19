package web

import (
	"context"
	"errors"
	"net/http"
	"s0counter/pkg/jwt_util"
	"strings"
)

var (
	ErrUnauthorized = errors.New("not authorized")
)

type Config struct {
	ApiKey    string
	JwtSecret string
	JwtID     string
	AppName   string
}

// ContextKey is used for storing values in context safely
type ContextKey string

const contextKeyUser ContextKey = "user"

// WithAuth is a middleware that checks if the request is authorized.
//   - If the request is not authorized, it returns a 401 Unauthorized response.
//   - If the request is authorized, it calls the next handler.
func WithAuth(h http.Handler, config Config) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			if len(config.ApiKey) > 0 && checkApiKey(r, config.ApiKey) {
				// update authenticated user in context and pass it to the next handler
				ctx := context.WithValue(r.Context(), contextKeyUser, "apikey")
				h.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if len(config.JwtSecret) > 0 && len(config.JwtID) > 0 {
				if claims, ok := checkJwtToken(r, config); ok {
					// update authenticated user in context and pass it to the next handler
					ctx := context.WithValue(r.Context(), contextKeyUser, claims.User)
					h.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// If neither API Key nor JWT is valid, return Unauthorized
			Encode(w, http.StatusUnauthorized, NewApiError(ErrUnauthorized))
		})
}

// checkApiKey checks if the request contains a valid API key.
//   - If the request does not contain a valid API key, it returns false.
//   - If the request contains a valid API key, it returns true.
//   - The API key is expected to be in the X-Api-Key header.
//   - The API key is compared to the given apiKey.
func checkApiKey(r *http.Request, apiKey string) bool {
	if key := r.Header.Get("X-Api-Key"); key != "" && key == apiKey {
		return true
	}
	return false
}

// checkJwtToken checks if the request contains a valid JWT token.
//   - If the request does not contain a valid JWT token, it returns false.
//   - If the request contains a valid JWT token, it returns true.
//   - The JWT token is expected to be in the Authorization header.
//   - The JWT token is validated using the given JWT secret and JWT ID.
//   - The claims of the JWT token are returned if the token is valid.
func checkJwtToken(r *http.Request, config Config) (*jwt_util.Claims, bool) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, false
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, false
	}

	claims, err := jwt_util.ValidateToken(parts[1], config.AppName, "auth", config.JwtID, config.JwtSecret)
	if err != nil {
		return nil, false
	}
	return claims, true
}
