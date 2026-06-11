package media

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ffmpegAvailable returns true if ffmpeg is on PATH, allowing tests to be
// skipped gracefully in environments without it.
func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func ffprobeAvailable() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

// makeSyntheticVideo creates a minimal silent video using ffmpeg for test purposes.
// Skips the test if ffmpeg is unavailable.
func makeSyntheticVideo(t *testing.T, destPath string, durationSec int) {
	t.Helper()
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH — skipping media file tests")
	}
	args := []string{
		"-f", "lavfi", "-i", fmt.Sprintf("color=c=blue:size=320x240:rate=10:duration=%d", durationSec),
		"-f", "lavfi", "-i", fmt.Sprintf("anullsrc=channel_layout=mono:sample_rate=44100:duration=%d", durationSec),
		"-c:v", "libx264", "-preset", "ultrafast", "-crf", "40",
		"-c:a", "aac", "-b:a", "32k",
		"-t", fmt.Sprintf("%d", durationSec),
		"-y", destPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to create synthetic video: %v\n%s", err, out)
	}
}

// --- ParseFFmpegTimecode ---

func TestParseFFmpegTimecode_SingleLine(t *testing.T) {
	line := "frame=  123 fps= 25 q=28.0 size=   256kB time=00:00:04.10 bitrate= 512.0kbits/s speed=1.02x"
	got := ParseFFmpegTimecode(line)
	want := float64(4*60+10) / 10.0 // 00:00:04.10 = 4.10s
	// 4 * 60 + 10 is wrong — it's 0h 0m 4.10s = 4.10s
	want = 4.10
	if abs(got-want) > 0.01 {
		t.Errorf("ParseFFmpegTimecode: got %.3f, want %.3f", got, want)
	}
}

func TestParseFFmpegTimecode_HoursAndMinutes(t *testing.T) {
	line := "frame= 9000 fps= 25 q=28.0 size=  8192kB time=01:02:03.45 bitrate=9999.9kbits/s speed=1.00x"
	got := ParseFFmpegTimecode(line)
	want := float64(1*3600 + 2*60 + 3 + 0.45) // 3723.45s
	if abs(got-want) > 0.01 {
		t.Errorf("ParseFFmpegTimecode: got %.3f, want %.3f", got, want)
	}
}

func TestParseFFmpegTimecode_MultipleLines(t *testing.T) {
	// A read buffer may contain several progress lines; we want the last one.
	chunk := "time=00:00:01.00 speed=1.0x\n" +
		"time=00:00:02.00 speed=1.0x\n" +
		"time=00:00:03.50 speed=1.0x\n"
	got := ParseFFmpegTimecode(chunk)
	if abs(got-3.50) > 0.01 {
		t.Errorf("ParseFFmpegTimecode multi-line: got %.3f, want 3.50", got)
	}
}

func TestParseFFmpegTimecode_NoMatch(t *testing.T) {
	got := ParseFFmpegTimecode("ffmpeg version 6.1 built with gcc")
	if got != -1 {
		t.Errorf("ParseFFmpegTimecode no match: expected -1, got %.3f", got)
	}
}

// --- parseFFprobeJSON ---

func TestParseFFprobeJSON_Duration(t *testing.T) {
	json := `{"streams":[{"codec_type":"video","width":1280,"height":720,"r_frame_rate":"30/1"}],"format":{"duration":"123.456"}}`
	result, err := parseFFprobeJSON([]byte(json))
	if err != nil {
		t.Fatalf("parseFFprobeJSON: %v", err)
	}
	if abs(result.DurationSeconds-123.456) > 0.001 {
		t.Errorf("duration: got %.3f, want 123.456", result.DurationSeconds)
	}
	if result.Width != 1280 {
		t.Errorf("width: got %d, want 1280", result.Width)
	}
	if result.Height != 720 {
		t.Errorf("height: got %d, want 720", result.Height)
	}
	if abs(result.FPS-30.0) > 0.01 {
		t.Errorf("fps: got %.2f, want 30.0", result.FPS)
	}
}

func TestParseFFprobeJSON_NumericDuration(t *testing.T) {
	// Some ffprobe outputs use bare numbers rather than quoted strings.
	json := `{"format":{"duration":60,"size":123456}}`
	result, err := parseFFprobeJSON([]byte(json))
	if err != nil {
		t.Fatalf("parseFFprobeJSON: %v", err)
	}
	if abs(result.DurationSeconds-60.0) > 0.001 {
		t.Errorf("duration: got %.3f, want 60.0", result.DurationSeconds)
	}
}

func TestParseFFprobeJSON_Empty(t *testing.T) {
	result, err := parseFFprobeJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("parseFFprobeJSON empty: %v", err)
	}
	if result.DurationSeconds != 0 {
		t.Errorf("expected zero duration on empty JSON, got %f", result.DurationSeconds)
	}
}

// --- Probe (integration, requires ffprobe) ---

func TestProbe_SyntheticFile(t *testing.T) {
	if !ffprobeAvailable() {
		t.Skip("ffprobe not on PATH — skipping Probe test")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	makeSyntheticVideo(t, videoPath, 5)

	result, err := Probe(videoPath)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.DurationSeconds < 4.5 || result.DurationSeconds > 6.0 {
		t.Errorf("probe duration: got %.3fs, expected ~5s", result.DurationSeconds)
	}
	if result.Width == 0 || result.Height == 0 {
		t.Errorf("probe dimensions: got %dx%d, expected non-zero", result.Width, result.Height)
	}
}

// --- GenerateThumbnail (integration, requires ffmpeg) ---

func TestGenerateThumbnail_SyntheticFile(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH — skipping thumbnail test")
	}

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "input.mp4")
	makeSyntheticVideo(t, videoPath, 10)

	posterPath := filepath.Join(dir, "poster.jpg")
	if err := GenerateThumbnail(videoPath, posterPath, 2.5); err != nil {
		t.Fatalf("GenerateThumbnail: %v", err)
	}

	info, err := os.Stat(posterPath)
	if err != nil {
		t.Fatalf("poster file not created: %v", err)
	}
	if info.Size() == 0 {
		t.Error("poster file is empty")
	}
}

func TestGenerateThumbnail_MissingInput(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not on PATH")
	}
	dir := t.TempDir()
	err := GenerateThumbnail("/nonexistent/file.mp4", filepath.Join(dir, "poster.jpg"), 1)
	if err == nil {
		t.Error("expected error for missing input, got nil")
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
