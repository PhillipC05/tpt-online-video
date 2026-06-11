// Package metrics exposes worker operational counters via a lightweight
// Prometheus-compatible text endpoint. It uses only sync/atomic — no external
// dependency — so the binary stays self-contained.
package metrics

import (
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
)

// WorkerMetrics tracks counters and gauges for the transcoding worker.
// All operations are safe for concurrent use.
type WorkerMetrics struct {
	JobsStarted    atomic.Int64
	JobsCompleted  atomic.Int64
	JobsFailed     atomic.Int64
	JobsPermanent  atomic.Int64 // failures routed straight to DLQ
	ActiveWorkers  atomic.Int32
	TotalDurationMs atomic.Int64 // cumulative processing time in milliseconds
}

// New returns an initialised WorkerMetrics ready for use.
func New() *WorkerMetrics { return &WorkerMetrics{} }

// RecordStart increments the started counter and active gauge.
func (m *WorkerMetrics) RecordStart() {
	m.JobsStarted.Add(1)
	m.ActiveWorkers.Add(1)
}

// RecordComplete decrements the active gauge and records elapsed milliseconds.
func (m *WorkerMetrics) RecordComplete(elapsedMs int64) {
	m.ActiveWorkers.Add(-1)
	m.JobsCompleted.Add(1)
	m.TotalDurationMs.Add(elapsedMs)
}

// RecordFailure decrements the active gauge and increments the failure counter.
// If permanent is true the permanent-failure counter is also incremented.
func (m *WorkerMetrics) RecordFailure(permanent bool) {
	m.ActiveWorkers.Add(-1)
	m.JobsFailed.Add(1)
	if permanent {
		m.JobsPermanent.Add(1)
	}
}

// WritePrometheus writes all metrics to w in Prometheus text exposition format.
func (m *WorkerMetrics) WritePrometheus(w io.Writer) {
	write := func(name, help, typ, value string) {
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n%s %s\n", name, help, name, typ, name, value)
	}

	write("worker_jobs_started_total",
		"Total number of transcoding jobs picked up by this worker.",
		"counter",
		fmt.Sprintf("%d", m.JobsStarted.Load()))

	write("worker_jobs_completed_total",
		"Total number of transcoding jobs completed successfully.",
		"counter",
		fmt.Sprintf("%d", m.JobsCompleted.Load()))

	write("worker_jobs_failed_total",
		"Total number of transcoding jobs that failed (transient + permanent).",
		"counter",
		fmt.Sprintf("%d", m.JobsFailed.Load()))

	write("worker_jobs_permanent_failure_total",
		"Total number of jobs routed to the dead-letter queue due to permanent failure.",
		"counter",
		fmt.Sprintf("%d", m.JobsPermanent.Load()))

	write("worker_active_jobs",
		"Number of jobs currently being processed.",
		"gauge",
		fmt.Sprintf("%d", m.ActiveWorkers.Load()))

	write("worker_processing_duration_ms_total",
		"Cumulative processing time across all completed jobs in milliseconds.",
		"counter",
		fmt.Sprintf("%d", m.TotalDurationMs.Load()))
}

// Handler returns an http.HandlerFunc that serves Prometheus-format metrics.
func (m *WorkerMetrics) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		m.WritePrometheus(w)
	}
}
