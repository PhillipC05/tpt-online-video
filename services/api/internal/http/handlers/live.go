package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
	svclive "github.com/tpt-online-video/services/api/internal/live"
)

// LiveHandler handles live stream HTTP requests.
type LiveHandler struct {
	logger     *slog.Logger
	service    *svclive.Service
	hookSecret string
}

// NewLiveHandler creates a new live stream handler.
func NewLiveHandler(logger *slog.Logger, db *pgxpool.Pool, svc *svclive.Service, hookSecret string) *LiveHandler {
	return &LiveHandler{
		logger:     logger,
		service:    svc,
		hookSecret: hookSecret,
	}
}

// ---------- Request / Response Types ----------

type CreateLiveStreamRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description,omitempty"`
	DVR          *bool  `json:"dvr_enabled,omitempty"`
	DVRWindowSec *int   `json:"dvr_window_seconds,omitempty"`
}

type UpdateLiveStreamRequest struct {
	Title       *string `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
}

type LiveStreamResponse struct {
	ID               string     `json:"id"`
	OwnerID          string     `json:"owner_id"`
	Title            string     `json:"title"`
	Description      string     `json:"description,omitempty"`
	Status           string     `json:"status"`
	RTMPURL          string     `json:"rtmp_url,omitempty"`
	HLSUrl           string     `json:"hls_url,omitempty"`
	WebRTCURL        string     `json:"webrtc_url,omitempty"`
	DVR              bool       `json:"dvr_enabled"`
	DVRWindowSeconds int        `json:"dvr_window_seconds"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	EndedAt          *time.Time `json:"ended_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type CreateLiveStreamResponse struct {
	Stream       LiveStreamResponse `json:"stream"`
	StreamKey    string             `json:"stream_key"`
	StreamKeyURL string             `json:"stream_key_url"`
}

// MediaMTX hook request payloads
type MediaMTXHookRequest struct {
	Action    string `json:"action"`
	ClientID  string `json:"clientId"`
	IP        string `json:"ip"`
	Type      string `json:"type"`
	Path      string `json:"path"`
	Protocol  string `json:"protocol"`
	ID        string `json:"id"`
	Query     string `json:"query"`
	StreamKey string `json:"streamKey"` // the path after /live/
}

type MediaMTXHookResponse struct {
	OK bool `json:"ok"`
}

// ---------- Helper ----------

func toLiveStreamResponse(s *svclive.Stream) LiveStreamResponse {
	return LiveStreamResponse{
		ID:               s.ID,
		OwnerID:          s.OwnerID,
		Title:            s.Title,
		Description:      s.Description,
		Status:           s.Status,
		RTMPURL:          s.RTMPURL,
		HLSUrl:           s.HLSUrl,
		WebRTCURL:        s.WebRTCURL,
		DVR:              s.DVR,
		DVRWindowSeconds: s.DVRWindowSeconds,
		StartedAt:        s.StartedAt,
		EndedAt:          s.EndedAt,
		CreatedAt:        s.CreatedAt,
	}
}

// checkHookSecret validates the X-Hook-Secret header.
func (h *LiveHandler) checkHookSecret(r *http.Request) bool {
	secret := r.Header.Get("X-Hook-Secret")
	return subtle.ConstantTimeCompare([]byte(secret), []byte(h.hookSecret)) == 1
}

// ---------- Handlers ----------

// POST /api/v1/live/streams
func (h *LiveHandler) CreateStream(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req CreateLiveStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cleanTitle, err := validateTitle(req.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, "title: "+err.Error())
		return
	}
	cleanDesc, err := validateDescription(req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, "description: "+err.Error())
		return
	}

	svcReq := svclive.CreateStreamRequest{
		Title:        cleanTitle,
		Description:  cleanDesc,
		DVR:          req.DVR,
		DVRWindowSec: req.DVRWindowSec,
	}

	result, err := h.service.Create(r.Context(), userID, svcReq)
	if err != nil {
		if strings.Contains(err.Error(), "already has an active stream") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		h.logger.Error("create live stream", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create live stream")
		return
	}

	writeJSON(w, http.StatusCreated, CreateLiveStreamResponse{
		Stream:       toLiveStreamResponse(result.Stream),
		StreamKey:    result.StreamKey,
		StreamKeyURL: result.StreamKeyURL,
	})
}

// GET /api/v1/live/streams/{streamID}
func (h *LiveHandler) GetStream(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream ID is required")
		return
	}

	callerID := middleware.GetUserID(r)

	stream, err := h.service.GetByID(r.Context(), streamID, callerID)
	if err != nil {
		h.logger.Error("get live stream", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if stream == nil {
		writeError(w, http.StatusNotFound, "live stream not found")
		return
	}

	writeJSON(w, http.StatusOK, toLiveStreamResponse(stream))
}

// PATCH /api/v1/live/streams/{streamID}
func (h *LiveHandler) UpdateStream(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	streamID := chi.URLParam(r, "streamID")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream ID is required")
		return
	}

	var req UpdateLiveStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title != nil {
		clean, err := validateTitle(*req.Title)
		if err != nil {
			writeError(w, http.StatusBadRequest, "title: "+err.Error())
			return
		}
		req.Title = &clean
	}
	if req.Description != nil {
		clean, err := validateDescription(*req.Description)
		if err != nil {
			writeError(w, http.StatusBadRequest, "description: "+err.Error())
			return
		}
		req.Description = &clean
	}

	stream, err := h.service.Update(r.Context(), streamID, userID, req.Title, req.Description)
	if err != nil {
		h.logger.Error("update live stream", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update live stream")
		return
	}
	if stream == nil {
		writeError(w, http.StatusNotFound, "live stream not found or not owned by user")
		return
	}

	writeJSON(w, http.StatusOK, toLiveStreamResponse(stream))
}

// DELETE /api/v1/live/streams/{streamID}
func (h *LiveHandler) DeleteStream(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	streamID := chi.URLParam(r, "streamID")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream ID is required")
		return
	}

	if err := h.service.Delete(r.Context(), streamID, userID); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "not owned") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.logger.Error("delete live stream", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to delete live stream")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "live stream deleted"})
}

// GET /api/v1/live/streams
func (h *LiveHandler) ListMyStreams(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	limit, offset := parsePagination(r)

	streams, err := h.service.ListByOwner(r.Context(), userID, limit, offset)
	if err != nil {
		h.logger.Error("list my live streams", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	resp := make([]LiveStreamResponse, len(streams))
	for i, s := range streams {
		resp[i] = toLiveStreamResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/live/streams/live
func (h *LiveHandler) ListLiveStreams(w http.ResponseWriter, r *http.Request) {
	streams, err := h.service.ListLive(r.Context())
	if err != nil {
		h.logger.Error("list live streams", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	resp := make([]LiveStreamResponse, len(streams))
	for i, s := range streams {
		resp[i] = toLiveStreamResponse(s)
	}

	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/live/streams/{streamID}/urls
func (h *LiveHandler) GetStreamURLs(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream ID is required")
		return
	}

	callerID := middleware.GetUserID(r)

	stream, err := h.service.GetByID(r.Context(), streamID, callerID)
	if err != nil {
		h.logger.Error("get live stream URLs", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if stream == nil {
		writeError(w, http.StatusNotFound, "live stream not found or not available")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"hls_url":    stream.HLSUrl,
		"webrtc_url": stream.WebRTCURL,
		"rtmp_url":   stream.RTMPURL,
	})
}

// ---------- MediaMTX Internal Webhooks ----------

// POST /api/v1/live/hooks/auth
func (h *LiveHandler) HookAuth(w http.ResponseWriter, r *http.Request) {
	if !h.checkHookSecret(r) {
		writeError(w, http.StatusUnauthorized, "invalid hook secret")
		return
	}

	var req MediaMTXHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.logger.Debug("mediamtx auth hook", "path", req.Path, "client", req.ClientID)

	// For RTMP, the path is /live/{streamKey}. Extract the stream key.
	// We always allow the auth check; the actual publish validation is in on-publish.
	writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: true})
}

// POST /api/v1/live/hooks/on-publish
func (h *LiveHandler) HookOnPublish(w http.ResponseWriter, r *http.Request) {
	if !h.checkHookSecret(r) {
		writeError(w, http.StatusUnauthorized, "invalid hook secret")
		return
	}

	var req MediaMTXHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.logger.Info("mediamtx publish hook", "path", req.Path, "streamKey", req.StreamKey)

	// Validate the stream key exists and is in a valid state
	streamKey := req.StreamKey
	if streamKey == "" {
		// Fallback: extract from path (RTMP: /live/{key})
		parts := strings.Split(strings.TrimPrefix(req.Path, "/live/"), "/")
		if len(parts) > 0 {
			streamKey = parts[0]
		}
	}

	if streamKey == "" {
		h.logger.Warn("publish hook missing stream key")
		writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: false})
		return
	}

	// Hash the stream key to look up the stream
	hash := svclive.HashStreamKey(streamKey)

	stream, err := h.service.DetectStart(r.Context(), hash)
	if err != nil {
		h.logger.Error("publish hook detect start", "error", err, "streamKeyHash", hash)
		writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: false})
		return
	}

	h.logger.Info("stream started", "stream_id", stream.ID, "title", stream.Title)
	writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: true})
}

// POST /api/v1/live/hooks/on-unpublish
func (h *LiveHandler) HookOnUnpublish(w http.ResponseWriter, r *http.Request) {
	if !h.checkHookSecret(r) {
		writeError(w, http.StatusUnauthorized, "invalid hook secret")
		return
	}

	var req MediaMTXHookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.logger.Info("mediamtx unpublish hook", "path", req.Path, "streamKey", req.StreamKey)

	streamKey := req.StreamKey
	if streamKey == "" {
		parts := strings.Split(strings.TrimPrefix(req.Path, "/live/"), "/")
		if len(parts) > 0 {
			streamKey = parts[0]
		}
	}

	if streamKey == "" {
		h.logger.Warn("unpublish hook missing stream key")
		writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: false})
		return
	}

	hash := svclive.HashStreamKey(streamKey)

	stream, err := h.service.DetectEnd(r.Context(), hash)
	if err != nil {
		h.logger.Error("unpublish hook detect end", "error", err, "streamKeyHash", hash)
		writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: false})
		return
	}

	h.logger.Info("stream ended", "stream_id", stream.ID, "title", stream.Title)
	writeJSON(w, http.StatusOK, MediaMTXHookResponse{OK: true})
}

// GET /api/v1/live/streams/{streamID}/dvr
func (h *LiveHandler) GetDVRInfo(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	if streamID == "" {
		writeError(w, http.StatusBadRequest, "stream ID is required")
		return
	}

	callerID := middleware.GetUserID(r)

	info, err := h.service.GetDVRInfo(r.Context(), streamID, callerID)
	if err != nil {
		h.logger.Error("get dvr info", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "live stream not found or not available")
		return
	}

	writeJSON(w, http.StatusOK, info)
}

// ---------- Helpers ----------

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}
	return
}