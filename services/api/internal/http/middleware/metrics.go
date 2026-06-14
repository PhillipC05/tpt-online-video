// Package middleware — API request metrics.
//
// APIMetrics uses only sync/atomic (no external dependency) and records the
// same counters the worker does: total requests, cumulative duration, 4xx/5xx
// error counts, and 401 auth failures.  A RequestLogger middleware writes one
// structured log line per completed request so individual log entries can be
// correlated by request_id.
package middleware

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// APIMetrics tracks HTTP-level counters for the API service.
// All fields are safe for concurrent use.
type APIMetrics struct {
	RequestsTotal   atomic.Int64
	DurationMsTotal atomic.Int64 // cumulative request duration in milliseconds
	Errors4xx       atomic.Int64
	Errors5xx       atomic.Int64
	AuthFailures    atomic.Int64 // 401 Unauthorized responses
}

// NewAPIMetrics returns an initialised APIMetrics ready for use.
func NewAPIMetrics() *APIMetrics { return &APIMetrics{} }

// metricsRecorder wraps a ResponseWriter to capture the status code written
// by the handler so the metrics middleware can bucket the response.
type metricsRecorder struct {
	http.ResponseWriter
	status    int
	wroteOnce bool
}

func (r *metricsRecorder) WriteHeader(code int) {
	if !r.wroteOnce {
		r.status = code
		r.wroteOnce = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *metricsRecorder) Write(b []byte) (int, error) {
	if !r.wroteOnce {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(b)
}

// Hijack delegates to the underlying writer so WebSocket handlers work correctly.
func (r *metricsRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not support hijacking")
}

// Flush delegates to the underlying writer for streaming responses.
func (r *metricsRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Middleware records per-request counters and cumulative duration.
func (m *APIMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		elapsed := time.Since(start).Milliseconds()

		m.RequestsTotal.Add(1)
		m.DurationMsTotal.Add(elapsed)

		switch {
		case rec.status == http.StatusUnauthorized:
			m.AuthFailures.Add(1)
			m.Errors4xx.Add(1)
		case rec.status >= 500:
			m.Errors5xx.Add(1)
		case rec.status >= 400:
			m.Errors4xx.Add(1)
		}
	})
}

// RequestLogger returns a middleware that emits one structured log line per
// completed request. The Chi request_id is included so log entries can be
// correlated back to a specific request.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &metricsRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			logger.Info("request",
				"request_id", chimiddleware.GetReqID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}

// WritePrometheus writes all API metrics in Prometheus text exposition format.
func (m *APIMetrics) WritePrometheus(w io.Writer) {
	write := func(name, help, typ, value string) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %s\n", name, help, name, typ, name, value)
	}

	write("api_requests_total",
		"Total number of HTTP requests handled by the API.",
		"counter",
		fmt.Sprintf("%d", m.RequestsTotal.Load()))

	write("api_request_duration_ms_total",
		"Cumulative HTTP request duration across all requests in milliseconds.",
		"counter",
		fmt.Sprintf("%d", m.DurationMsTotal.Load()))

	write("api_errors_4xx_total",
		"Total number of HTTP 4xx responses.",
		"counter",
		fmt.Sprintf("%d", m.Errors4xx.Load()))

	write("api_errors_5xx_total",
		"Total number of HTTP 5xx responses.",
		"counter",
		fmt.Sprintf("%d", m.Errors5xx.Load()))

	write("api_auth_failures_total",
		"Total number of HTTP 401 Unauthorized responses.",
		"counter",
		fmt.Sprintf("%d", m.AuthFailures.Load()))
}

// Handler returns an http.HandlerFunc that serves Prometheus-format API metrics.
func (m *APIMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		m.WritePrometheus(w)
	}
}
