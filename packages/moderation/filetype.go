package moderation

import (
	"fmt"
	"strings"
)

// MIME type categories for validation.
var (
	// AllowedVideoMIMEs lists the video MIME types accepted for upload.
	AllowedVideoMIMEs = map[string]bool{
		"video/mp4":                true,
		"video/webm":               true,
		"video/ogg":                true,
		"video/quicktime":          true, // .mov
		"video/x-msvideo":          true, // .avi
		"video/x-matroska":         true, // .mkv
		"video/x-ms-wmv":           true, // .wmv
		"video/x-flv":              true, // .flv
		"video/mpeg":               true, // .mpeg
		"video/3gpp":               true, // .3gp
		"video/mp2t":               true, // .ts
	}

	// AllowedVideoExtensions maps file extensions to their expected MIME types.
	AllowedVideoExtensions = map[string]string{
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".ogg":  "video/ogg",
		".ogv":  "video/ogg",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".wmv":  "video/x-ms-wmv",
		".flv":  "video/x-flv",
		".mpeg": "video/mpeg",
		".mpg":  "video/mpeg",
		".3gp":  "video/3gpp",
		".ts":   "video/mp2t",
		".m2ts": "video/mp2t",
	}
)

// ValidateFileType checks whether the given filename and MIME type are allowed.
// It returns an error if the file type is not supported.
func ValidateFileType(filename string, mimeType string) error {
	ext := strings.ToLower(extFromFilename(filename))

	expectedMIME, extOK := AllowedVideoExtensions[ext]
	mimeOK := AllowedVideoMIMEs[mimeType]

	if !extOK && !mimeOK {
		return fmt.Errorf("unsupported file type: extension %q and MIME %q are not allowed", ext, mimeType)
	}
	if extOK && !mimeOK {
		return fmt.Errorf("unsupported MIME type %q for extension %q", mimeType, ext)
	}
	if !extOK && mimeOK {
		return fmt.Errorf("unsupported extension %q for MIME type %q", ext, mimeType)
	}
	if expectedMIME != "" && mimeType != expectedMIME {
		return fmt.Errorf("MIME type %q does not match expected type %q for extension %q", mimeType, expectedMIME, ext)
	}

	return nil
}

// ValidateFileSize checks whether the given file size is within allowed limits.
func ValidateFileSize(byteSize int64, maxBytes int64) error {
	if byteSize <= 0 {
		return fmt.Errorf("file size must be positive")
	}
	if maxBytes > 0 && byteSize > maxBytes {
		return fmt.Errorf("file size %d exceeds maximum allowed %d", byteSize, maxBytes)
	}
	return nil
}

// extFromFilename extracts the file extension from a filename.
func extFromFilename(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return ""
	}
	return filename[idx:]
}