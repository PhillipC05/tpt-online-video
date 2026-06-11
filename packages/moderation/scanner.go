package moderation

import (
	"context"
	"io"
)

// ScanResult describes the outcome of a file scan.
type ScanResult struct {
	Infected bool
	Threat   string // e.g. "Win.Trojan.Generic", empty if clean
}

// Scanner defines the interface for malware/virus scanning.
// Implementations may be no-op (development), ClamAV, or a cloud API.
type Scanner interface {
	// Scan reads the content from r and returns a scan result.
	// r is expected to be consumed; callers should reset/re-read if needed.
	Scan(ctx context.Context, name string, r io.Reader) (*ScanResult, error)

	// Name returns a human-readable name of the scanner implementation.
	Name() string
}