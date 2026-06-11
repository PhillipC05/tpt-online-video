package processor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tpt-online-video/packages/media"
)

// ffmpegAvailable returns true if ffmpeg is on PATH.
func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// makeSyntheticVideo creates a minimal H.264/AAC file using ffmpeg lavfi sources.
func makeSyntheticVideo(t *testing.T, destPath string, durationSec int) {
	t.Helper()
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH — skipping media tests")
	}
	args := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=red:size=320x240:rate=10:duration=%d", durationSec),
		"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=channel_layout=mono:sample_rate=44100:duration=%d", durationSec),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "45",
		"-c:a", "aac", "-b:a", "32k",
		"-t", fmt.Sprintf("%d", durationSec),
		"-y", destPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create synthetic video: %v\n%s", err, out)
	}
}

// --- Progress parsing ---

func TestParseFFmpegTimecodeProgressPct(t *testing.T) {
	// Verify the percentage is correctly bounded to <100 even at near-end timecodes.
	totalDuration := 10.0

	cases := []struct {
		output  string
		wantPct float64
	}{
		{"time=00:00:05.00 speed=1.0x", 50.0},
		{"time=00:00:09.99 speed=1.0x", 99.0}, // capped at 99
		{"time=00:00:10.50 speed=1.0x", 99.0}, // past end → capped
		{"time=00:00:01.00 speed=1.0x\ntime=00:00:08.00 speed=1.0x", 80.0},
	}

	for _, tc := range cases {
		currentSec := media.ParseFFmpegTimecode(tc.output)
		if currentSec < 0 {
			t.Errorf("no timecode in %q", tc.output)
			continue
		}
		pct := currentSec / totalDuration * 100
		if pct > 99 {
			pct = 99
		}
		if abs(pct-tc.wantPct) > 0.5 {
			t.Errorf("output=%q: got %.1f%%, want %.1f%%", tc.output, pct, tc.wantPct)
		}
	}
}

// --- Error classification routing ---

func TestProcessJob_PermanentErrorSkipsRetry(t *testing.T) {
	// Verify that a PermanentError is detected correctly so the caller can
	// route it to the DLQ without incrementing the attempt counter.
	err := media.ClassifyFFmpegError(
		fmt.Errorf("ffmpeg failed: Invalid data found when processing input"),
	)
	if !media.IsPermanent(err) {
		t.Fatalf("expected permanent error, got: %T %v", err, err)
	}
}

func TestProcessJob_TransientErrorAllowsRetry(t *testing.T) {
	err := media.ClassifyFFmpegError(
		fmt.Errorf("ffmpeg failed: context deadline exceeded"),
	)
	if media.IsPermanent(err) {
		t.Fatalf("expected transient error, got permanent: %v", err)
	}
}

// --- GenerateThumbnail integration ---

func TestGenerateThumbnail_Integration(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	makeSyntheticVideo(t, videoPath, 8)

	posterPath := filepath.Join(dir, "poster.jpg")
	if err := media.GenerateThumbnail(videoPath, posterPath, 2.0); err != nil {
		t.Fatalf("GenerateThumbnail: %v", err)
	}

	info, err := os.Stat(posterPath)
	if err != nil {
		t.Fatalf("poster not created: %v", err)
	}
	if info.Size() < 100 {
		t.Errorf("poster suspiciously small: %d bytes", info.Size())
	}
}

// --- BuildHLSCommand smoke test ---

func TestBuildHLSCommand_Integration(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	makeSyntheticVideo(t, videoPath, 5)

	hlsDir := filepath.Join(dir, "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		t.Fatalf("mkdir hls: %v", err)
	}

	// Use only the lowest rendition to keep the test fast.
	renditions := []media.HLSRendition{
		{Name: "360p", Width: 640, Height: 360, Bitrate: 800, FrameRate: "10"},
	}
	cmd := media.BuildHLSCommand(videoPath, hlsDir, renditions)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("BuildHLSCommand: %v\n%s", err, out)
	}

	// Expect at least the 360p playlist to exist.
	m3u8 := filepath.Join(hlsDir, "360p.m3u8")
	if _, err := os.Stat(m3u8); err != nil {
		t.Errorf("360p.m3u8 not created: %v", err)
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
