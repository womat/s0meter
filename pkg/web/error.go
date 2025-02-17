package web

import (
	"fmt"
	"log/slog"
	"net/http"
)

// ApiError is a generic api error response.
type ApiError struct {
	Error string `json:"error"`
}

// NewApiError creates a new ApiError from an error.
func NewApiError(err error) ApiError {
	if err == nil {
		return ApiError{"unknown error"}
	}
	return ApiError{Error: err.Error()}
}

// WriteError writes an Error response. Optional reason is only logged and not send as response.
func WriteError(w http.ResponseWriter, r *http.Request, status int, err error, reason ...error) {
	slog.Error(fmt.Sprintf("%s %s -> Status: %d, Error: %s", r.Method, r.URL.Path, status, err.Error()))

	log := slog.With("method", r.Method, "path", r.URL.Path, "query", r.URL.Query(), "status", status, "error", err)
	if len(reason) > 0 {
		log = log.With("reason", reason[0])
	}
	if r.Method != "GET" {
		log = log.With("body", r.Body)
	}
	log.Debug("API error")

	Encode(w, status, NewApiError(err))
}
