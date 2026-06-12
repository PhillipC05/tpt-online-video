package live

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// ServiceConfig holds configuration for the live stream service.
type ServiceConfig struct {
	RTMPBaseURL      string // e.g., rtmp://localhost:1935/live
	HLSBaseURL       string // e.g., http://localhost:8888/live
	WebRTCBaseURL    string // e.g., http://localhost:8889/live
	DVRDefaultWindow int    // seconds, default 900 (15 minutes)
	HLSDirectory     string // local path where MediaMTX writes HLS segments, e.g. /var/mediamtx/hls
}

// Service handles live stream business logic.
type Service struct {
	repo   *Repository
	logger *slog.Logger
	cfg    ServiceConfig
}

// NewService creates a new live stream service.
func NewService(repo *Repository, logger *slog.Logger, cfg ServiceConfig) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
		cfg:    cfg,
	}
}

// CreateStreamRequest contains the fields needed to create a live stream.
type CreateStreamRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	DVR          *bool  `json:"dvr_enabled,omitempty"`
	DVRWindowSec *int   `json:"dvr_window_seconds,omitempty"`
}

// CreateStreamResponse is returned after creating a stream.
type CreateStreamResponse struct {
	Stream       *Stream `json:"stream"`
	StreamKey    string  `json:"stream_key"` // plaintext, shown once
	StreamKeyURL string  `json:"stream_key_url"`
	RTMPURL      string  `json:"rtmp_url"`
	HLSUrl       string  `json:"hls_url"`
	WebRTCURL    string  `json:"webrtc_url"`
}

// Create creates a new live stream with a generated key.
func (s *Service) Create(ctx context.Context, ownerID string, req CreateStreamRequest) (*CreateStreamResponse, error) {
	// Check for existing active stream
	existing, err := s.repo.GetActiveByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("check existing stream: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("user already has an active stream (id=%s, status=%s)", existing.ID, existing.Status)
	}

	// Generate stream key
	plaintextKey, hash, err := GenerateStreamKey()
	if err != nil {
		return nil, fmt.Errorf("generate stream key: %w", err)
	}

	dvr := true
	dvrWindow := s.cfg.DVRDefaultWindow
	if req.DVR != nil {
		dvr = *req.DVR
	}
	if req.DVRWindowSec != nil && *req.DVRWindowSec > 0 {
		dvrWindow = *req.DVRWindowSec
	}

	stream := &Stream{
		OwnerID:          ownerID,
		Title:            req.Title,
		Description:      req.Description,
		StreamKeyHash:    hash,
		Status:           "idle",
		DVR:              dvr,
		DVRWindowSeconds: dvrWindow,
	}

	created, err := s.repo.Create(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("create stream: %w", err)
	}

	// Build URLs
	rtmpURL := fmt.Sprintf("%s/%s", s.cfg.RTMPBaseURL, plaintextKey)
	hlsURL := fmt.Sprintf("%s/%s/index.m3u8", s.cfg.HLSBaseURL, plaintextKey)
	webrtcURL := fmt.Sprintf("%s/%s", s.cfg.WebRTCBaseURL, plaintextKey)
	streamKeyURL := rtmpURL

	// Store URLs in DB
	if err := s.repo.SetStreamURLs(ctx, created.ID, rtmpURL, hlsURL, webrtcURL); err != nil {
		s.logger.Error("set stream URLs", "error", err, "stream_id", created.ID)
	}

	// Refresh from DB to get URLs
	created, _ = s.repo.GetByID(ctx, created.ID)

	return &CreateStreamResponse{
		Stream:       created,
		StreamKey:    plaintextKey,
		StreamKeyURL: streamKeyURL,
		RTMPURL:      rtmpURL,
		HLSUrl:       hlsURL,
		WebRTCURL:    webrtcURL,
	}, nil
}

// GetByID returns a stream by ID (with owner check).
func (s *Service) GetByID(ctx context.Context, id string, callerID string) (*Stream, error) {
	stream, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}
	// Non-owners can only see streams that are live or ended
	if stream.OwnerID != callerID && stream.Status != "live" && stream.Status != "ended" {
		return nil, nil
	}
	return stream, nil
}

// GetPublicByID returns a stream by ID (anyone can see live/ended streams).
func (s *Service) GetPublicByID(ctx context.Context, id string) (*Stream, error) {
	stream, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}
	if stream.Status != "live" && stream.Status != "ended" {
		return nil, nil
	}
	return stream, nil
}

// Update updates a stream's title/description.
func (s *Service) Update(ctx context.Context, id, ownerID string, title, description *string) (*Stream, error) {
	return s.repo.Update(ctx, id, ownerID, title, description)
}

// Delete deletes a stream.
func (s *Service) Delete(ctx context.Context, id, ownerID string) error {
	deleted, err := s.repo.Delete(ctx, id, ownerID)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("stream not found or not owned by user")
	}
	return nil
}

// ListByOwner returns streams for a user.
func (s *Service) ListByOwner(ctx context.Context, ownerID string, limit, offset int) ([]*Stream, error) {
	return s.repo.ListByOwner(ctx, ownerID, limit, offset)
}

// ListLive returns all currently live streams.
func (s *Service) ListLive(ctx context.Context) ([]*Stream, error) {
	return s.repo.ListLive(ctx)
}

// DetectStart marks a stream as live. Called by the live helper when an RTMP push is detected.
func (s *Service) DetectStart(ctx context.Context, streamKeyHash string) (*Stream, error) {
	stream, err := s.repo.GetByStreamKeyHash(ctx, streamKeyHash)
	if err != nil {
		return nil, fmt.Errorf("lookup stream by key hash: %w", err)
	}
	if stream == nil {
		return nil, fmt.Errorf("no stream found for key hash")
	}
	if stream.Status == "live" {
		// Already live, no-op
		return stream, nil
	}
	if stream.Status != "idle" {
		return nil, fmt.Errorf("stream %s cannot transition to live from status %s", stream.ID, stream.Status)
	}

	now := time.Now().UTC()
	if err := s.repo.UpdateStatus(ctx, stream.ID, "live", &now, nil); err != nil {
		return nil, fmt.Errorf("update stream status to live: %w", err)
	}

	stream.Status = "live"
	stream.StartedAt = &now
	return stream, nil
}

// DetectEnd marks a stream as ended. Called by the live helper when RTMP push disconnects.
func (s *Service) DetectEnd(ctx context.Context, streamKeyHash string) (*Stream, error) {
	stream, err := s.repo.GetByStreamKeyHash(ctx, streamKeyHash)
	if err != nil {
		return nil, fmt.Errorf("lookup stream by key hash: %w", err)
	}
	if stream == nil {
		return nil, fmt.Errorf("no stream found for key hash")
	}
	if stream.Status == "ended" || stream.Status == "ending" {
		return stream, nil
	}

	now := time.Now().UTC()
	// Transition through "ending" briefly before "ended"
	if err := s.repo.UpdateStatus(ctx, stream.ID, "ending", nil, nil); err != nil {
		return nil, fmt.Errorf("update stream status to ending: %w", err)
	}
	if err := s.repo.UpdateStatus(ctx, stream.ID, "ended", nil, &now); err != nil {
		return nil, fmt.Errorf("update stream status to ended: %w", err)
	}

	stream.Status = "ended"
	stream.EndedAt = &now
	return stream, nil
}

// GetDVRInfo returns DVR seek-range metadata for a stream.
func (s *Service) GetDVRInfo(ctx context.Context, streamID, callerID string) (*DVRInfo, error) {
	stream, err := s.GetByID(ctx, streamID, callerID)
	if err != nil {
		return nil, err
	}
	if stream == nil {
		return nil, nil
	}
	info := ComputeDVRInfo(stream)
	return &info, nil
}

// GetStreamKeyHash returns the hash for a stream key (for the live helper).
func (s *Service) GetStreamKeyHash(ctx context.Context, streamID string) (string, error) {
	stream, err := s.repo.GetByID(ctx, streamID)
	if err != nil {
		return "", err
	}
	if stream == nil {
		return "", fmt.Errorf("stream not found")
	}
	return stream.StreamKeyHash, nil
}