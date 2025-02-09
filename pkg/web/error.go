package web

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
