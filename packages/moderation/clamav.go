package moderation

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ClamAVScanner scans file content via a running clamd daemon over TCP.
// It uses the INSTREAM command of the clamd protocol.
type ClamAVScanner struct {
	addr    string
	timeout time.Duration
}

// NewClamAVScanner returns a scanner that connects to clamd at addr (e.g. "localhost:3310").
func NewClamAVScanner(addr string) *ClamAVScanner {
	return &ClamAVScanner{
		addr:    addr,
		timeout: 30 * time.Second,
	}
}

func (s *ClamAVScanner) Name() string { return "clamav" }

// Scan streams r to clamd via INSTREAM and returns the scan result.
// Callers should not reuse r after Scan returns.
func (s *ClamAVScanner) Scan(ctx context.Context, name string, r io.Reader) (*ScanResult, error) {
	deadline := time.Now().Add(s.timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}

	conn, err := net.DialTimeout("tcp", s.addr, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("clamav: connect %s: %w", s.addr, err)
	}
	defer conn.Close()
	conn.SetDeadline(deadline)

	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return nil, fmt.Errorf("clamav: send command: %w", err)
	}

	// Stream data as length-prefixed chunks (4-byte big-endian uint32 + data).
	buf := make([]byte, 8192)
	lenBuf := make([]byte, 4)
	for {
		n, readErr := r.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(lenBuf, uint32(n))
			if _, err := conn.Write(lenBuf); err != nil {
				return nil, fmt.Errorf("clamav: write length: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return nil, fmt.Errorf("clamav: write data: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("clamav: read input: %w", readErr)
		}
	}

	// Terminate stream with a zero-length chunk.
	binary.BigEndian.PutUint32(lenBuf, 0)
	if _, err := conn.Write(lenBuf); err != nil {
		return nil, fmt.Errorf("clamav: send terminator: %w", err)
	}

	resp, err := io.ReadAll(conn)
	if err != nil {
		return nil, fmt.Errorf("clamav: read response: %w", err)
	}

	line := strings.TrimSpace(string(resp))
	switch {
	case strings.HasSuffix(line, " OK"):
		return &ScanResult{Infected: false}, nil
	case strings.HasSuffix(line, " FOUND"):
		// "stream: <virusname> FOUND"
		threat := ""
		if parts := strings.SplitN(line, ": ", 2); len(parts) == 2 {
			threat = strings.TrimSuffix(parts[1], " FOUND")
		}
		return &ScanResult{Infected: true, Threat: threat}, nil
	default:
		return nil, fmt.Errorf("clamav: unexpected response: %q", line)
	}
}
