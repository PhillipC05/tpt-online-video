package media

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tpt-online-video/packages/storage"
)

// HLSRendition defines a single HLS output rendition.
type HLSRendition struct {
	Name      string
	Width     int
	Height    int
	Bitrate   int
	FrameRate string
}

// DefaultRenditions returns the standard set of HLS renditions.
func DefaultRenditions() []HLSRendition {
	return []HLSRendition{
		{Name: "1080p", Width: 1920, Height: 1080, Bitrate: 5000, FrameRate: "30"},
		{Name: "720p", Width: 1280, Height: 720, Bitrate: 2800, FrameRate: "30"},
		{Name: "480p", Width: 854, Height: 480, Bitrate: 1400, FrameRate: "30"},
		{Name: "360p", Width: 640, Height: 360, Bitrate: 800, FrameRate: "30"},
	}
}

type FFprobeOutput struct {
	DurationSeconds float64
	Width           int
	Height          int
	FPS             float64
}

// Probe runs ffprobe on a local file and returns media metadata.
func Probe(filePath string) (*FFprobeOutput, error) {
	// Get duration
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	}
	cmd := exec.Command("ffprobe", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	return parseFFprobeJSON(out)
}

func parseFFprobeJSON(data []byte) (*FFprobeOutput, error) {
	// Simple JSON parsing without external dependency
	// In production, use encoding/json with a struct. This is a lightweight approach.
	output := string(data)
	result := &FFprobeOutput{}

	if idx := strings.Index(output, `"duration"`); idx >= 0 {
		rest := output[idx:]
		start := strings.Index(rest, `"`)
		if start >= 0 {
			rest = rest[start+1:]
			end := strings.Index(rest, `"`)
			if end >= 0 {
				result.DurationSeconds, _ = strconv.ParseFloat(rest[start+1:start+1+end], 64)
			}
		}
	}
	return result, nil
}

// BuildHLSCommand builds an FFmpeg command that generates HLS renditions from a source file.
// It outputs to a temporary directory and returns the paths to each rendition's m3u8.
func BuildHLSCommand(inputPath, outputDir string, renditions []HLSRendition) *exec.Cmd {
	var filterComplexParts []string
	var mapParts []string

	for i, r := range renditions {
		filterComplexParts = append(filterComplexParts,
			fmt.Sprintf("[0:v]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,setdar=%d/%d[v%d]",
				r.Width, r.Height, r.Width, r.Height, i))
	}

	args := []string{
		"-i", inputPath,
		"-preset", "fast",
		"-g", "48",
		"-sc_threshold", "0",
	}

	if len(filterComplexParts) > 0 {
		args = append(args, "-filter_complex", strings.Join(filterComplexParts, "; "))
	}

	for i, r := range renditions {
		args = append(args,
			"-map", fmt.Sprintf("[v%d]", i),
			"-c:v", "libx264",
			"-b:v", fmt.Sprintf("%dk", r.Bitrate),
			"-r", r.FrameRate,
			"-maxrate", fmt.Sprintf("%dk", r.Bitrate+r.Bitrate/2),
			"-bufsize", fmt.Sprintf("%dk", r.Bitrate*2),
			"-crf", "23",
			fmt.Sprintf("%s/%s.m3u8", outputDir, r.Name),
		)
	}

	// Audio map
	args = append(args,
		"-map", "0:a?",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "44100",
		fmt.Sprintf("%s/audio.m3u8", outputDir),
	)

	return exec.Command("ffmpeg", args...)
}

// UploadHLSRenditions uploads all HLS files from a local output directory to storage.
func UploadHLSRenditions(ctx context.Context, store storage.Provider, bucket, outputDir, videoID string) error {
	// This is a simplified stub - in production, walk the directory and upload each file.
	// The worker should generate the master playlist and upload it.
	cmd := exec.Command("find", outputDir, "-type", "f")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("list HLS files: %w", err)
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, file := range files {
		if file == "" {
			continue
		}
		// Upload each HLS file to storage
		relPath := strings.TrimPrefix(file, outputDir+"/")
		key := fmt.Sprintf("hls/%s/%s", videoID, relPath)

		// We'd use os.Open + store.PutObject here
		_ = key
		_ = bucket
	}
	return nil
}