package live

import (
	"context"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DVRInfo describes the seek range available to a viewer for a given stream.
type DVRInfo struct {
	Enabled          bool    `json:"dvr_enabled"`
	WindowSeconds    int     `json:"dvr_window_seconds"`
	SeekableSeconds  float64 `json:"seekable_seconds"`  // how far back a viewer can seek right now
	AtLiveEdge       bool    `json:"at_live_edge"`      // hint: player starts at live edge
	HLSDVRURL        string  `json:"hls_dvr_url"`       // same as hls_url; included for explicitness
}

// ComputeDVRInfo returns DVR metadata for a stream.
func ComputeDVRInfo(s *Stream) DVRInfo {
	if !s.DVR {
		return DVRInfo{Enabled: false}
	}

	seekable := 0.0
	if s.Status == "live" && s.StartedAt != nil {
		elapsed := time.Since(*s.StartedAt).Seconds()
		if elapsed > float64(s.DVRWindowSeconds) {
			seekable = float64(s.DVRWindowSeconds)
		} else {
			seekable = elapsed
		}
	} else if (s.Status == "ended" || s.Status == "ending") && s.StartedAt != nil && s.EndedAt != nil {
		duration := s.EndedAt.Sub(*s.StartedAt).Seconds()
		if duration > float64(s.DVRWindowSeconds) {
			seekable = float64(s.DVRWindowSeconds)
		} else {
			seekable = duration
		}
	}

	return DVRInfo{
		Enabled:         true,
		WindowSeconds:   s.DVRWindowSeconds,
		SeekableSeconds: seekable,
		AtLiveEdge:      true,
		HLSDVRURL:       s.HLSUrl,
	}
}

// DVRCleaner removes MediaMTX HLS segments from disk after a stream's DVR
// retention window expires. Run it as a background goroutine via Run().
type DVRCleaner struct {
	repo         *Repository
	logger       *slog.Logger
	hlsDirectory string // local filesystem path, e.g. /var/mediamtx/hls
	hlsBaseURL   string // e.g. http://mediamtx:8888
	interval     time.Duration
}

// NewDVRCleaner creates a cleaner that scans every interval.
func NewDVRCleaner(repo *Repository, logger *slog.Logger, hlsDirectory, hlsBaseURL string, interval time.Duration) *DVRCleaner {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &DVRCleaner{
		repo:         repo,
		logger:       logger,
		hlsDirectory: hlsDirectory,
		hlsBaseURL:   hlsBaseURL,
		interval:     interval,
	}
}

// Run blocks, cleaning up expired DVR segment directories on each tick.
// Cancel ctx to stop.
func (c *DVRCleaner) Run(ctx context.Context) {
	if c.hlsDirectory == "" {
		c.logger.Info("dvr cleaner: hlsDirectory not configured, skipping disk cleanup")
		return
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.logger.Info("dvr cleaner started", "interval", c.interval, "hlsDirectory", c.hlsDirectory)

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("dvr cleaner stopped")
			return
		case <-ticker.C:
			c.sweep(ctx)
		}
	}
}

// sweep finds ended streams whose DVR retention has expired and deletes their
// segment directories.
func (c *DVRCleaner) sweep(ctx context.Context) {
	streams, err := c.repo.ListEndedWithDVR(ctx)
	if err != nil {
		c.logger.Error("dvr cleaner: list ended streams", "error", err)
		return
	}

	for _, s := range streams {
		if !s.DVR || s.EndedAt == nil {
			continue
		}

		retentionEnd := s.EndedAt.Add(time.Duration(s.DVRWindowSeconds) * time.Second)
		if time.Now().UTC().Before(retentionEnd) {
			continue // retention window still active
		}

		dir := c.segmentDir(s)
		if dir == "" {
			continue
		}

		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			c.logger.Error("dvr cleaner: remove segments", "stream_id", s.ID, "dir", dir, "error", err)
		} else {
			c.logger.Info("dvr cleaner: removed segments", "stream_id", s.ID, "dir", dir)
		}

		if err := c.repo.MarkDVRCleaned(ctx, s.ID); err != nil {
			c.logger.Error("dvr cleaner: mark cleaned", "stream_id", s.ID, "error", err)
		}
	}
}

// segmentDir derives the MediaMTX segment directory for a stream from its
// hls_url. MediaMTX writes to {hlsDirectory}/{streamPath}/, where streamPath
// is the URL path component after the host (e.g. "live/stk_abc123").
func (c *DVRCleaner) segmentDir(s *Stream) string {
	if s.HLSUrl == "" {
		return ""
	}

	u, err := url.Parse(s.HLSUrl)
	if err != nil {
		return ""
	}

	// Drop trailing /index.m3u8 to get the stream directory path.
	streamPath := strings.TrimSuffix(u.Path, "/index.m3u8")
	streamPath = strings.TrimPrefix(streamPath, "/")

	return filepath.Join(c.hlsDirectory, filepath.FromSlash(streamPath))
}
