package media

import (
	"errors"
	"testing"
)

func TestPermanentError_Wrapping(t *testing.T) {
	cause := errors.New("underlying cause")
	err := Permanent(cause, "test reason")

	if !IsPermanent(err) {
		t.Error("IsPermanent should be true for a PermanentError")
	}
	if IsTransient(err) {
		t.Error("IsTransient should be false for a PermanentError")
	}
	if !errors.Is(err, cause) {
		t.Error("PermanentError should unwrap to cause")
	}
}

func TestTransientError_Wrapping(t *testing.T) {
	cause := errors.New("network timeout")
	err := Transient(cause)

	if IsPermanent(err) {
		t.Error("IsPermanent should be false for a TransientError")
	}
	if !IsTransient(err) {
		t.Error("IsTransient should be true for a TransientError")
	}
	if !errors.Is(err, cause) {
		t.Error("TransientError should unwrap to cause")
	}
}

func TestIsPermanent_NestedChain(t *testing.T) {
	inner := Permanent(errors.New("root"), "codec unsupported")

	// A plain new error with the same message is NOT permanent (no chain).
	plain := errors.New("outer: " + inner.Error())
	if IsPermanent(plain) {
		t.Error("plain error with matching message should not be IsPermanent")
	}

	// errors.Join preserves the wrapped error chain.
	joined := errors.Join(inner)
	if !IsPermanent(joined) {
		t.Error("errors.Join should preserve the permanent chain")
	}
}

func TestClassifyFFmpegError_Permanent(t *testing.T) {
	permanentMessages := []string{
		"ffmpeg failed: exit status 1: Invalid data found when processing input",
		"ffmpeg failed: moov atom not found",
		"ffmpeg failed: Invalid NAL unit size",
		"ffmpeg failed: codec not currently supported in this container",
		"ffmpeg failed: Could not open codec for video stream",
	}

	for _, msg := range permanentMessages {
		err := ClassifyFFmpegError(errors.New(msg))
		if !IsPermanent(err) {
			t.Errorf("expected permanent for %q, got transient", msg)
		}
	}
}

func TestClassifyFFmpegError_Transient(t *testing.T) {
	transientMessages := []string{
		"ffmpeg failed: connection reset by peer",
		"ffmpeg failed: context deadline exceeded",
		"ffmpeg failed: exit status 1",
		"ffmpeg failed: temporary file error",
	}

	for _, msg := range transientMessages {
		err := ClassifyFFmpegError(errors.New(msg))
		if IsPermanent(err) {
			t.Errorf("expected transient for %q, got permanent", msg)
		}
	}
}

func TestClassifyFFmpegError_Nil(t *testing.T) {
	if ClassifyFFmpegError(nil) != nil {
		t.Error("ClassifyFFmpegError(nil) should return nil")
	}
}

func TestClassifyStorageError_Permanent(t *testing.T) {
	notFoundMessages := []string{
		"get object: not found",
		"get object: NoSuchKey: key does not exist",
		"get object: 404 Not Found",
		"get object: nosuchkey",
	}

	for _, msg := range notFoundMessages {
		err := ClassifyStorageError(errors.New(msg))
		if !IsPermanent(err) {
			t.Errorf("expected permanent for %q, got transient", msg)
		}
	}
}

func TestClassifyStorageError_Transient(t *testing.T) {
	transientMessages := []string{
		"get object: connection refused",
		"get object: i/o timeout",
		"get object: dial tcp: connection reset",
	}

	for _, msg := range transientMessages {
		err := ClassifyStorageError(errors.New(msg))
		if IsPermanent(err) {
			t.Errorf("expected transient for %q, got permanent", msg)
		}
	}
}
