package media

import (
	"errors"
	"strings"
)

// PermanentError wraps an error that should not be retried — the job should be
// moved directly to the dead-letter queue regardless of remaining attempts.
type PermanentError struct {
	Cause  error
	Reason string
}

func (e *PermanentError) Error() string {
	return "permanent failure (" + e.Reason + "): " + e.Cause.Error()
}

func (e *PermanentError) Unwrap() error { return e.Cause }

// TransientError wraps an error that is expected to be temporary and safe to retry.
type TransientError struct {
	Cause error
}

func (e *TransientError) Error() string { return "transient failure: " + e.Cause.Error() }
func (e *TransientError) Unwrap() error  { return e.Cause }

// IsPermanent reports whether err (or any error in its chain) is a PermanentError.
func IsPermanent(err error) bool {
	var p *PermanentError
	return errors.As(err, &p)
}

// IsTransient reports whether err (or any error in its chain) is a TransientError.
func IsTransient(err error) bool {
	var t *TransientError
	return errors.As(err, &t)
}

// Permanent wraps err as a PermanentError with the given human-readable reason.
func Permanent(err error, reason string) *PermanentError {
	return &PermanentError{Cause: err, Reason: reason}
}

// Transient wraps err as a TransientError.
func Transient(err error) *TransientError {
	return &TransientError{Cause: err}
}

// ffmpegPermanentPhrases contains substrings that appear in FFmpeg stderr when
// the input is irreparably corrupt or uses an unsupported format.
var ffmpegPermanentPhrases = []string{
	"Invalid data found when processing input",
	"moov atom not found",
	"No such file or directory",
	"Invalid NAL unit size",
	"Decoder (codec",
	"Unknown encoder",
	"Encoder",
	"not found",
	"codec not currently supported",
	"unsupported codec",
	"no video streams",
	"no streams",
	"Could not open codec",
	"not supported",
}

// ClassifyFFmpegError inspects the FFmpeg error message and returns either a
// PermanentError (corrupt/unsupported input) or a TransientError (everything else).
func ClassifyFFmpegError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	for _, phrase := range ffmpegPermanentPhrases {
		if strings.Contains(msg, strings.ToLower(phrase)) {
			return Permanent(err, phrase)
		}
	}
	return Transient(err)
}

// ClassifyStorageError maps storage-layer errors to permanent/transient.
// A 404 / object-not-found is permanent (the source file is gone).
// All other errors are treated as transient (network blip, timeout, etc.).
func ClassifyStorageError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") ||
		strings.Contains(msg, "no such key") ||
		strings.Contains(msg, "nosuchkey") ||
		strings.Contains(msg, "404") {
		return Permanent(err, "object not found in storage")
	}
	return Transient(err)
}
