package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/jackc/pgx/v5"
)

// Recoverer is a middleware that recovers from panics and returns a 500 error.
func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("panic recovered",
						"error", rec,
						"stack", string(debug.Stack()),
						"path", r.URL.Path,
						"method", r.Method,
					)
					WriteInternalError(w, "an unexpected error occurred")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// NotFoundHandler returns a 404 for unmatched routes.
func NotFoundHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteNotFound(w, "route not found")
	})
}

// MethodNotAllowedHandler returns a 405 for disallowed methods.
func MethodNotAllowedHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
	})
}

// ErrorMapping contains the mapping from domain errors to HTTP status codes.
type ErrorMapping struct {
	Err error
	Status int
	Code   string
}

// ErrorHandlerMiddleware maps known errors to proper HTTP responses.
func ErrorHandlerMiddleware(logger *slog.Logger, mappings ...ErrorMapping) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// This middleware wraps the response writer to intercept status codes
			// and map errors. We use a custom wrapper.
			erw := &errorResponseWriter{
				ResponseWriter: w,
				logger:         logger,
				mappings:       mappings,
				wroteHeader:    false,
			}
			next.ServeHTTP(erw, r)
		})
	}
}

type errorResponseWriter struct {
	http.ResponseWriter
	logger      *slog.Logger
	mappings    []ErrorMapping
	statusCode  int
	wroteHeader bool
}

func (w *errorResponseWriter) WriteHeader(statusCode int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.statusCode = statusCode

	// If status is 5xx, log the error
	if statusCode >= 500 {
		w.logger.Error("server error",
			"status", statusCode,
		)
	}

	w.ResponseWriter.WriteHeader(statusCode)
}

// MapDBError maps database errors to appropriate HTTP errors.
func MapDBError(logger *slog.Logger, err error) (int, string, string) {
	if err == nil {
		return http.StatusOK, "", ""
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return http.StatusNotFound, "NOT_FOUND", "resource not found"
	}
	logger.Error("database error", "error", err)
	return http.StatusInternalServerError, "DATABASE_ERROR", "database error"
}