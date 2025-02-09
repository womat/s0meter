package web

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// logResponse is a wrapper around http.ResponseWriter that logs the response status and body.
//   - Status is the HTTP status code.
//   - Body is the response body.
//   - ResponseWriter is the original http.ResponseWriter.
type logResponse struct {
	Status int
	Body   []byte
	http.ResponseWriter
}

// newLogResponse creates a new logResponse.
//   - w is the original http.ResponseWriter.
//   - Returns a new logResponse.
func newLogResponse(w http.ResponseWriter) *logResponse {
	return &logResponse{ResponseWriter: w}
}

// WithLogging is a middleware that logs the request and response.
//   - h is the next handler.
//   - Returns a new handler that logs the request and response.
//   - It logs the request method, URL, and body.
//   - It logs the response status and body.
func WithLogging(h http.Handler, logger *slog.Logger) http.Handler {

	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {

			// Wrap the response writer.
			lr := newLogResponse(w)
			h.ServeHTTP(lr, r)

			// Copy request body to buffer
			var reqBody bytes.Buffer
			_, _ = io.Copy(&reqBody, r.Body)

			// Log the request and response.
			logger.Debug(fmt.Sprintf("%s %s -> %d", r.Method, r.URL.String(), lr.Status),
				slog.Group("request", slog.String("method", r.Method), slog.String("url", r.URL.String()), slog.String("body", reqBody.String())),
				slog.Group("response", slog.Int("status", lr.Status), slog.String("body", string(lr.Body))),
			)

		},
	)
}
