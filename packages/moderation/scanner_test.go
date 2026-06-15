package moderation

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

// --- NopScanner ---

func TestNopScanner_AlwaysClean(t *testing.T) {
	s := NewNopScanner()

	result, err := s.Scan(context.Background(), "malware.exe", strings.NewReader("dangerous content"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.Infected {
		t.Error("NopScanner should always report clean, got Infected=true")
	}
	if result.Threat != "" {
		t.Errorf("NopScanner should have empty Threat, got %q", result.Threat)
	}
}

func TestNopScanner_Name(t *testing.T) {
	s := NewNopScanner()
	if s.Name() != "noop" {
		t.Errorf("expected name %q, got %q", "noop", s.Name())
	}
}

func TestNopScanner_EmptyReader(t *testing.T) {
	s := NewNopScanner()
	result, err := s.Scan(context.Background(), "empty.bin", bytes.NewReader(nil))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.Infected {
		t.Error("expected clean result for empty reader")
	}
}

// --- ClamAVScanner (mock TCP server) ---

// mockClamdServer listens on a random TCP port and returns a fixed response.
// It implements the INSTREAM handshake: reads the command + chunks, then writes response.
func mockClamdServer(t *testing.T, response string) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	addr := ln.Addr().String()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// Read and discard the "zINSTREAM\x00" command and all chunk data
		// until we see the terminating zero-length chunk.
		cmd := make([]byte, 10) // "zINSTREAM\x00"
		io.ReadFull(conn, cmd)

		lenBuf := make([]byte, 4)
		for {
			if _, err := io.ReadFull(conn, lenBuf); err != nil {
				return
			}
			chunkLen := binary.BigEndian.Uint32(lenBuf)
			if chunkLen == 0 {
				break
			}
			io.CopyN(io.Discard, conn, int64(chunkLen))
		}

		fmt.Fprint(conn, response)
	}()

	return addr
}

func TestClamAVScanner_CleanFile(t *testing.T) {
	addr := mockClamdServer(t, "stream: OK\n")
	s := NewClamAVScanner(addr)

	result, err := s.Scan(context.Background(), "clean.mp4", strings.NewReader("video data"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.Infected {
		t.Error("expected clean result, got Infected=true")
	}
	if result.Threat != "" {
		t.Errorf("expected empty Threat, got %q", result.Threat)
	}
}

func TestClamAVScanner_InfectedFile(t *testing.T) {
	addr := mockClamdServer(t, "stream: Win.Trojan.Generic FOUND\n")
	s := NewClamAVScanner(addr)

	result, err := s.Scan(context.Background(), "virus.exe", strings.NewReader("EICAR"))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !result.Infected {
		t.Error("expected Infected=true, got false")
	}
	if result.Threat != "Win.Trojan.Generic" {
		t.Errorf("expected Threat=%q, got %q", "Win.Trojan.Generic", result.Threat)
	}
}

func TestClamAVScanner_UnexpectedResponse(t *testing.T) {
	addr := mockClamdServer(t, "stream: ERROR something went wrong\n")
	s := NewClamAVScanner(addr)

	_, err := s.Scan(context.Background(), "file.bin", strings.NewReader("data"))
	if err == nil {
		t.Error("expected error for unexpected response, got nil")
	}
}

func TestClamAVScanner_ConnectionRefused(t *testing.T) {
	// Use an address that is not listening.
	s := NewClamAVScanner("127.0.0.1:19999")

	_, err := s.Scan(context.Background(), "file.bin", strings.NewReader("data"))
	if err == nil {
		t.Error("expected error when connection is refused, got nil")
	}
}

func TestClamAVScanner_Name(t *testing.T) {
	s := NewClamAVScanner("localhost:3310")
	if s.Name() != "clamav" {
		t.Errorf("expected name %q, got %q", "clamav", s.Name())
	}
}

func TestClamAVScanner_LargeFile(t *testing.T) {
	addr := mockClamdServer(t, "stream: OK\n")
	s := NewClamAVScanner(addr)

	// 64 KB of data — exercises the chunking loop
	data := bytes.Repeat([]byte("A"), 64*1024)
	result, err := s.Scan(context.Background(), "large.bin", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if result.Infected {
		t.Error("expected clean result for large file")
	}
}
