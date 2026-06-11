package media

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tpt-online-video/packages/storage"
)

// ErrNoSubtitleStream is returned by ExtractSubtitles when the source file
// contains no subtitle stream.
var ErrNoSubtitleStream = errors.New("no subtitle stream found")

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
	result := &FFprobeOutput{}

	// Extract "duration": "123.456" or "duration": 123.456
	if v, ok := extractJSONStringOrNumber(string(data), "duration"); ok {
		result.DurationSeconds, _ = strconv.ParseFloat(v, 64)
	}
	// Width and height come from video stream fields.
	if v, ok := extractJSONStringOrNumber(string(data), "width"); ok {
		w, _ := strconv.Atoi(v)
		result.Width = w
	}
	if v, ok := extractJSONStringOrNumber(string(data), "height"); ok {
		h, _ := strconv.Atoi(v)
		result.Height = h
	}
	// r_frame_rate is "30/1" or "30000/1001"
	if v, ok := extractJSONStringOrNumber(string(data), "r_frame_rate"); ok {
		parts := strings.SplitN(v, "/", 2)
		if len(parts) == 2 {
			num, _ := strconv.ParseFloat(parts[0], 64)
			den, _ := strconv.ParseFloat(parts[1], 64)
			if den != 0 {
				result.FPS = num / den
			}
		}
	}
	return result, nil
}

// extractJSONStringOrNumber finds the value of a JSON key in raw JSON text.
// It handles both string values ("key": "val") and numeric values ("key": 123).
// Returns the raw value string and true if found.
func extractJSONStringOrNumber(json, key string) (string, bool) {
	needle := `"` + key + `"`
	idx := strings.Index(json, needle)
	if idx < 0 {
		return "", false
	}
	rest := json[idx+len(needle):]
	// Skip whitespace and colon.
	i := 0
	for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == ':' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	if i >= len(rest) {
		return "", false
	}
	rest = rest[i:]
	if rest[0] == '"' {
		// Quoted string: find closing quote.
		end := strings.Index(rest[1:], `"`)
		if end < 0 {
			return "", false
		}
		return rest[1 : 1+end], true
	}
	// Numeric or other: read until delimiter.
	end := strings.IndexAny(rest, ",}\n\r ")
	if end < 0 {
		end = len(rest)
	}
	val := strings.TrimSpace(rest[:end])
	if val == "" {
		return "", false
	}
	return val, true
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

// ParseFFmpegTimecode parses the wall-clock position from a chunk of FFmpeg stderr.
// FFmpeg writes lines like: "frame=  123 fps= 25 … time=00:01:23.45 bitrate=…"
// Returns the latest parsed position in seconds, or -1 if none was found.
func ParseFFmpegTimecode(output string) float64 {
	latest := float64(-1)
	remaining := output
	for {
		idx := strings.Index(remaining, "time=")
		if idx < 0 {
			break
		}
		timeStr := remaining[idx+5:]
		end := strings.IndexAny(timeStr, " \t\n\r")
		if end > 0 {
			timeStr = timeStr[:end]
		}
		var h, m int
		var s float64
		if n, _ := fmt.Sscanf(timeStr, "%d:%d:%f", &h, &m, &s); n == 3 {
			latest = float64(h*3600+m*60) + s
		}
		remaining = remaining[idx+5:]
	}
	return latest
}

// GenerateThumbnail extracts a single frame from inputPath at seekSec seconds and
// writes a JPEG to destPath. Quality 2 is near-lossless (1=best, 31=worst).
func GenerateThumbnail(inputPath, destPath string, seekSec float64) error {
	args := []string{
		"-ss", fmt.Sprintf("%.3f", seekSec),
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		"-y",
		destPath,
	}
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg thumbnail: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// BuildDASHCommand builds an FFmpeg command that generates a MPEG-DASH manifest and
// fragmented MP4 segments for all renditions. All output files land in outputDir;
// the top-level manifest is outputDir/manifest.mpd.
func BuildDASHCommand(inputPath, outputDir string, renditions []HLSRendition) *exec.Cmd {
	var filterComplexParts []string

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
			"-profile:v", "high",
			"-b:v", fmt.Sprintf("%dk", r.Bitrate),
			"-r", r.FrameRate,
			"-maxrate", fmt.Sprintf("%dk", r.Bitrate+r.Bitrate/2),
			"-bufsize", fmt.Sprintf("%dk", r.Bitrate*2),
			"-crf", "23",
		)
	}

	args = append(args,
		"-map", "0:a?",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ar", "44100",
		"-f", "dash",
		"-seg_duration", "4",
		"-use_template", "1",
		"-use_timeline", "1",
		"-init_seg_name", "init_$RepresentationID$.mp4",
		"-media_seg_name", "chunk_$RepresentationID$_$Number%05d$.m4s",
		"-y",
		filepath.Join(outputDir, "manifest.mpd"),
	)

	return exec.Command("ffmpeg", args...)
}

// ExtractSubtitles extracts the first subtitle stream from inputPath as a WebVTT
// file at destPath. Returns ErrNoSubtitleStream if no subtitle track is present.
func ExtractSubtitles(inputPath, destPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-map", "0:s:0",
		"-f", "webvtt",
		"-y",
		destPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := string(out)
		if strings.Contains(s, "matches no streams") ||
			strings.Contains(s, "Stream specifier 0:s:0") ||
			strings.Contains(s, "no subtitle") {
			return ErrNoSubtitleStream
		}
		return fmt.Errorf("extract subtitles: %w: %s", err, strings.TrimSpace(s))
	}
	return nil
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