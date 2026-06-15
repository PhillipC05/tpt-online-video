package live

import (
	"testing"
	"time"
)

// ptr helpers
func ptrTime(t time.Time) *time.Time { return &t }

// --- ComputeDVRInfo ---

func TestComputeDVRInfo_DVRDisabled(t *testing.T) {
	s := &Stream{DVR: false}
	info := ComputeDVRInfo(s)
	if info.Enabled {
		t.Error("expected DVR disabled")
	}
	if info.SeekableSeconds != 0 {
		t.Errorf("expected SeekableSeconds=0, got %f", info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_LiveStream_WithinWindow(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	s := &Stream{
		DVR:              true,
		Status:           "live",
		DVRWindowSeconds: 900, // 15 minutes
		StartedAt:        ptrTime(startedAt),
	}
	info := ComputeDVRInfo(s)

	if !info.Enabled {
		t.Error("expected DVR enabled")
	}
	if info.WindowSeconds != 900 {
		t.Errorf("expected WindowSeconds=900, got %d", info.WindowSeconds)
	}
	// 5 minutes elapsed, window is 15 min → seekable = ~300s
	if info.SeekableSeconds < 290 || info.SeekableSeconds > 310 {
		t.Errorf("expected SeekableSeconds≈300, got %f", info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_LiveStream_ExceedsWindow(t *testing.T) {
	// Stream started 30 minutes ago, window is 15 minutes
	startedAt := time.Now().Add(-30 * time.Minute)
	s := &Stream{
		DVR:              true,
		Status:           "live",
		DVRWindowSeconds: 900,
		StartedAt:        ptrTime(startedAt),
	}
	info := ComputeDVRInfo(s)

	if info.SeekableSeconds != float64(s.DVRWindowSeconds) {
		t.Errorf("expected SeekableSeconds=%d (capped at window), got %f", s.DVRWindowSeconds, info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_EndedStream_WithinWindow(t *testing.T) {
	startedAt := time.Now().Add(-10 * time.Minute)
	endedAt := time.Now().Add(-2 * time.Minute)
	duration := endedAt.Sub(startedAt).Seconds() // ~480s

	s := &Stream{
		DVR:              true,
		Status:           "ended",
		DVRWindowSeconds: 900,
		StartedAt:        ptrTime(startedAt),
		EndedAt:          ptrTime(endedAt),
	}
	info := ComputeDVRInfo(s)

	if !info.Enabled {
		t.Error("expected DVR enabled for ended stream")
	}
	// 8 minutes duration < 15 minute window → seekable == duration
	if info.SeekableSeconds < duration-5 || info.SeekableSeconds > duration+5 {
		t.Errorf("expected SeekableSeconds≈%f, got %f", duration, info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_EndedStream_ExceedsWindow(t *testing.T) {
	startedAt := time.Now().Add(-40 * time.Minute)
	endedAt := time.Now().Add(-5 * time.Minute)
	s := &Stream{
		DVR:              true,
		Status:           "ended",
		DVRWindowSeconds: 900,
		StartedAt:        ptrTime(startedAt),
		EndedAt:          ptrTime(endedAt),
	}
	info := ComputeDVRInfo(s)

	if info.SeekableSeconds != float64(s.DVRWindowSeconds) {
		t.Errorf("expected SeekableSeconds capped at %d, got %f", s.DVRWindowSeconds, info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_LiveStream_NoStartedAt(t *testing.T) {
	s := &Stream{
		DVR:              true,
		Status:           "live",
		DVRWindowSeconds: 900,
		StartedAt:        nil,
	}
	info := ComputeDVRInfo(s)
	if info.SeekableSeconds != 0 {
		t.Errorf("expected SeekableSeconds=0 when StartedAt is nil, got %f", info.SeekableSeconds)
	}
}

func TestComputeDVRInfo_AtLiveEdge(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	s := &Stream{
		DVR:              true,
		Status:           "live",
		DVRWindowSeconds: 900,
		StartedAt:        ptrTime(startedAt),
		HLSUrl:           "http://mediamtx:8888/live/stk_abc123/index.m3u8",
	}
	info := ComputeDVRInfo(s)

	if !info.AtLiveEdge {
		t.Error("expected AtLiveEdge=true")
	}
	if info.HLSDVRURL != s.HLSUrl {
		t.Errorf("expected HLSDVRURL=%q, got %q", s.HLSUrl, info.HLSDVRURL)
	}
}

// --- DVRCleaner.segmentDir ---

func TestDVRCleaner_SegmentDir(t *testing.T) {
	cleaner := &DVRCleaner{
		hlsDirectory: "/var/mediamtx/hls",
		hlsBaseURL:   "http://mediamtx:8888",
	}

	cases := []struct {
		name     string
		hlsURL   string
		contains string // expected substring in path
		empty    bool
	}{
		{
			name:     "standard HLS URL",
			hlsURL:   "http://mediamtx:8888/live/stk_abc123/index.m3u8",
			contains: "stk_abc123",
		},
		{
			name:  "empty HLS URL",
			hlsURL: "",
			empty: true,
		},
		{
			name:  "invalid URL",
			hlsURL: "://bad-url",
			empty: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := &Stream{HLSUrl: c.hlsURL}
			dir := cleaner.segmentDir(s)
			if c.empty {
				if dir != "" {
					t.Errorf("expected empty dir, got %q", dir)
				}
				return
			}
			if dir == "" {
				t.Error("expected non-empty dir")
			}
			if c.contains != "" && !containsStr(dir, c.contains) {
				t.Errorf("expected dir to contain %q, got %q", c.contains, dir)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}())
}
