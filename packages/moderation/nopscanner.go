package moderation

import (
	"context"
	"io"
)

// NopScanner is a no-op scanner that always reports clean.
// Used in development and testing.
type NopScanner struct{}

func NewNopScanner() *NopScanner {
	return &NopScanner{}
}

func (s *NopScanner) Scan(_ context.Context, _ string, _ io.Reader) (*ScanResult, error) {
	return &ScanResult{Infected: false, Threat: ""}, nil
}

func (s *NopScanner) Name() string {
	return "noop"
}