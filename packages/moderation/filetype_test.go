package moderation

import (
	"testing"
)

// --- ValidateFileType ---

func TestValidateFileType_ValidCombinations(t *testing.T) {
	cases := []struct {
		filename string
		mime     string
	}{
		{"video.mp4", "video/mp4"},
		{"video.webm", "video/webm"},
		{"video.ogg", "video/ogg"},
		{"video.ogv", "video/ogg"},
		{"video.mov", "video/quicktime"},
		{"video.avi", "video/x-msvideo"},
		{"video.mkv", "video/x-matroska"},
		{"video.wmv", "video/x-ms-wmv"},
		{"video.flv", "video/x-flv"},
		{"video.mpeg", "video/mpeg"},
		{"video.mpg", "video/mpeg"},
		{"video.3gp", "video/3gpp"},
		{"video.ts", "video/mp2t"},
		{"video.m2ts", "video/mp2t"},
	}
	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			if err := ValidateFileType(c.filename, c.mime); err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestValidateFileType_UnknownExtensionAndMIME(t *testing.T) {
	if err := ValidateFileType("document.pdf", "application/pdf"); err == nil {
		t.Error("expected error for unsupported file type, got nil")
	}
}

func TestValidateFileType_ValidExtensionWrongMIME(t *testing.T) {
	// .mp4 extension but wrong MIME
	if err := ValidateFileType("video.mp4", "application/octet-stream"); err == nil {
		t.Error("expected error for mismatched MIME type, got nil")
	}
}

func TestValidateFileType_ValidMIMEUnknownExtension(t *testing.T) {
	// Valid MIME but extension not in allowed set
	if err := ValidateFileType("video.unknownext", "video/mp4"); err == nil {
		t.Error("expected error for unknown extension, got nil")
	}
}

func TestValidateFileType_MIMEExtensionMismatch(t *testing.T) {
	// .mp4 expects video/mp4, not video/webm
	if err := ValidateFileType("video.mp4", "video/webm"); err == nil {
		t.Error("expected error for mismatched MIME/extension combination, got nil")
	}
}

func TestValidateFileType_NoExtension(t *testing.T) {
	if err := ValidateFileType("noextension", "video/mp4"); err == nil {
		t.Error("expected error for filename with no extension")
	}
}

func TestValidateFileType_CaseInsensitiveExtension(t *testing.T) {
	// ValidateFileType lowercases the extension, so .MP4 should work.
	if err := ValidateFileType("VIDEO.MP4", "video/mp4"); err != nil {
		t.Errorf("expected nil for uppercase extension, got %v", err)
	}
}

// --- ValidateFileSize ---

func TestValidateFileSize_Valid(t *testing.T) {
	if err := ValidateFileSize(1024, 10*1024*1024); err != nil {
		t.Errorf("expected nil for valid size, got %v", err)
	}
}

func TestValidateFileSize_ZeroSize(t *testing.T) {
	if err := ValidateFileSize(0, 10*1024*1024); err == nil {
		t.Error("expected error for zero size, got nil")
	}
}

func TestValidateFileSize_NegativeSize(t *testing.T) {
	if err := ValidateFileSize(-1, 10*1024*1024); err == nil {
		t.Error("expected error for negative size, got nil")
	}
}

func TestValidateFileSize_ExceedsMax(t *testing.T) {
	if err := ValidateFileSize(100, 50); err == nil {
		t.Error("expected error when size exceeds max, got nil")
	}
}

func TestValidateFileSize_ExactlyMax(t *testing.T) {
	if err := ValidateFileSize(50, 50); err != nil {
		t.Errorf("expected nil for size exactly at max, got %v", err)
	}
}

func TestValidateFileSize_NoMaxLimit(t *testing.T) {
	// maxBytes == 0 means no limit
	if err := ValidateFileSize(999999999, 0); err != nil {
		t.Errorf("expected nil when maxBytes=0 (no limit), got %v", err)
	}
}

// --- extFromFilename ---

func TestExtFromFilename(t *testing.T) {
	cases := []struct {
		filename string
		expected string
	}{
		{"video.mp4", ".mp4"},
		{"my.video.mp4", ".mp4"},
		{"noextension", ""},
		{".hidden", ".hidden"},
	}
	for _, c := range cases {
		t.Run(c.filename, func(t *testing.T) {
			got := extFromFilename(c.filename)
			if got != c.expected {
				t.Errorf("extFromFilename(%q) = %q, want %q", c.filename, got, c.expected)
			}
		})
	}
}
