package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// APIResponse is the standard response envelope for all API responses.
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// APIError represents a structured error in the API response.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteOK sends a 200 success response with data.
func WriteOK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteCreated sends a 201 success response with data.
func WriteCreated(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, APIResponse{
		Success: true,
		Data:    data,
	})
}

// WriteError sends an error response with the given status code and error details.
func WriteError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// WriteValidationError sends a 422 validation error response.
func WriteValidationError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", message)
}

// WriteNotFound sends a 404 error response.
func WriteNotFound(w http.ResponseWriter, message string) {
	if message == "" {
		message = "resource not found"
	}
	WriteError(w, http.StatusNotFound, "NOT_FOUND", message)
}

// WriteUnauthorized sends a 401 error response.
func WriteUnauthorized(w http.ResponseWriter, message string) {
	if message == "" {
		message = "unauthorized"
	}
	WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// WriteForbidden sends a 403 error response.
func WriteForbidden(w http.ResponseWriter, message string) {
	if message == "" {
		message = "forbidden"
	}
	WriteError(w, http.StatusForbidden, "FORBIDDEN", message)
}

// WriteConflict sends a 409 error response.
func WriteConflict(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusConflict, "CONFLICT", message)
}

// WriteInternalError sends a 500 error response.
func WriteInternalError(w http.ResponseWriter, message string) {
	if message == "" {
		message = "internal server error"
	}
	WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

// WriteTooManyRequests sends a 429 error response.
func WriteTooManyRequests(w http.ResponseWriter, message string) {
	if message == "" {
		message = "too many requests"
	}
	WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", message)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Warn("write json response", "error", err)
	}
}