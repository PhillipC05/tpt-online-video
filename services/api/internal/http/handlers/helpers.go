package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

// writeJSON sends a success response with the standard envelope.
func writeJSON(w http.ResponseWriter, status int, data any) {
	switch status {
	case http.StatusOK:
		middleware.WriteOK(w, data)
	case http.StatusCreated:
		middleware.WriteCreated(w, data)
	default:
		middleware.WriteOK(w, data)
	}
}

// writeError sends an error response with the standard envelope.
func writeError(w http.ResponseWriter, status int, message string) {
	switch status {
	case http.StatusBadRequest:
		middleware.WriteValidationError(w, message)
	case http.StatusUnauthorized:
		middleware.WriteUnauthorized(w, message)
	case http.StatusForbidden:
		middleware.WriteForbidden(w, message)
	case http.StatusNotFound:
		middleware.WriteNotFound(w, message)
	case http.StatusConflict:
		middleware.WriteConflict(w, message)
	case http.StatusTooManyRequests:
		middleware.WriteTooManyRequests(w, message)
	default:
		middleware.WriteInternalError(w, message)
	}
}

// handleDBError maps database errors to appropriate HTTP responses.
func handleDBError(logger *slog.Logger, w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, pgx.ErrNoRows) {
		middleware.WriteNotFound(w, "resource not found")
		return true
	}
	logger.Error("database error", "error", err)
	middleware.WriteInternalError(w, "database error")
	return true
}