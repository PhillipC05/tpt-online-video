package metrics

import (
	"strings"
	"testing"
)

func TestRecordStart(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordStart()

	if got := m.JobsStarted.Load(); got != 2 {
		t.Errorf("JobsStarted: got %d, want 2", got)
	}
	if got := m.ActiveWorkers.Load(); got != 2 {
		t.Errorf("ActiveWorkers after 2 starts: got %d, want 2", got)
	}
}

func TestRecordComplete(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordComplete(500)

	if got := m.JobsCompleted.Load(); got != 1 {
		t.Errorf("JobsCompleted: got %d, want 1", got)
	}
	if got := m.ActiveWorkers.Load(); got != 0 {
		t.Errorf("ActiveWorkers after complete: got %d, want 0", got)
	}
	if got := m.TotalDurationMs.Load(); got != 500 {
		t.Errorf("TotalDurationMs: got %d, want 500", got)
	}
}

func TestRecordFailure_Transient(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordFailure(false)

	if got := m.JobsFailed.Load(); got != 1 {
		t.Errorf("JobsFailed: got %d, want 1", got)
	}
	if got := m.JobsPermanent.Load(); got != 0 {
		t.Errorf("JobsPermanent should be 0 for transient failure, got %d", got)
	}
	if got := m.ActiveWorkers.Load(); got != 0 {
		t.Errorf("ActiveWorkers after failure: got %d, want 0", got)
	}
}

func TestRecordFailure_Permanent(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordFailure(true)

	if got := m.JobsFailed.Load(); got != 1 {
		t.Errorf("JobsFailed: got %d, want 1", got)
	}
	if got := m.JobsPermanent.Load(); got != 1 {
		t.Errorf("JobsPermanent: got %d, want 1", got)
	}
}

func TestWritePrometheus_ContainsAllMetrics(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordComplete(1234)
	m.RecordStart()
	m.RecordFailure(true)

	var buf strings.Builder
	m.WritePrometheus(&buf)
	output := buf.String()

	required := []string{
		"worker_jobs_started_total",
		"worker_jobs_completed_total",
		"worker_jobs_failed_total",
		"worker_jobs_permanent_failure_total",
		"worker_active_jobs",
		"worker_processing_duration_ms_total",
		"# TYPE",
		"# HELP",
	}
	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("metrics output missing %q", s)
		}
	}
}

func TestWritePrometheus_Values(t *testing.T) {
	m := New()
	m.RecordStart()
	m.RecordStart()
	m.RecordComplete(100)

	var buf strings.Builder
	m.WritePrometheus(&buf)
	output := buf.String()

	// JobsStarted should be 2.
	if !strings.Contains(output, "worker_jobs_started_total 2") {
		t.Errorf("expected 'worker_jobs_started_total 2' in output:\n%s", output)
	}
	// JobsCompleted should be 1.
	if !strings.Contains(output, "worker_jobs_completed_total 1") {
		t.Errorf("expected 'worker_jobs_completed_total 1' in output:\n%s", output)
	}
	// One worker still active.
	if !strings.Contains(output, "worker_active_jobs 1") {
		t.Errorf("expected 'worker_active_jobs 1' in output:\n%s", output)
	}
}
