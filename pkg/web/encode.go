package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Encode writes the response as JSON.
//   - w is the response writer.
//   - status is the HTTP status code.
//   - v is the value to Encode as JSON.
//   - If encoding fails, it writes an ApiError with http status InternalServerError.
func Encode[T any](w http.ResponseWriter, status int, v T) {

	w.Header().Set("Content-Type", "application/json")

	resp, err := json.Marshal(v)
	if err != nil {
		resp, _ = json.Marshal(NewApiError(err))
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(status)
	}

	_, _ = w.Write(resp)
}

// decode reads the request Body as JSON.
//   - r is the request.
//   - T is the type to decode the JSON into.
//   - If decoding fails, it returns an error.
func decode[T any](r *http.Request) (T, error) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		return v, fmt.Errorf("decode json failed: %w", err)
	}
	return v, nil
}
