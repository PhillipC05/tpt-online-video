package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

type VideoHandler struct {
	logger    *slog.Logger
	db        *pgxpool.Pool
	redis     *redis.Client
	storage   storage.Provider
	mediaBase string
}

func NewVideoHandler(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, mediaBase string) *VideoHandler {
	return &VideoHandler{
		logger:    logger,
		db:        db,
		redis:     redis,
		storage:   store,
		mediaBase: mediaBase,
	}
}

type VideoResponse struct {
	ID              string              `json:"id"`
	Title           string              `json:"title"`
	Description     string              `json:"description,omitempty"`
	Visibility      string              `json:"visibility"`
	Status          string              `json:"status"`
	DurationSeconds *float64            `json:"duration_seconds,omitempty"`
	Width           *int                `json:"width,omitempty"`
	Height          *int                `json:"height,omitempty"`
	ViewCount       int64               `json:"view_count"`
	CreatedAt       string              `json:"created_at"`
	PublishedAt     *string             `json:"published_at,omitempty"`
	ThumbnailURL    string              `json:"thumbnail_url,omitempty"`
	HLSManifestURL  string              `json:"hls_manifest_url,omitempty"`
	DASHManifestURL string              `json:"dash_manifest_url,omitempty"`
	Renditions      []RenditionInfo     `json:"renditions,omitempty"`
	SubtitleTracks  []SubtitleTrackInfo `json:"subtitle_tracks,omitempty"`
	Owner           *OwnerInfo          `json:"owner,omitempty"`
}

type SubtitleTrackInfo struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	URL      string `json:"url"`
}

type RenditionInfo struct {
	Name   string `json:"name"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

type OwnerInfo struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (h *VideoHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	var id, title, description, visibility, status, ownerID, displayName string
	var viewCount int64
	var createdAt, publishedAt *time.Time
	var durationSeconds *float64
	var width, height *int
	var hasDash, hasSubtitles bool

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, COALESCE(v.description, ''), v.visibility::text, v.status::text,
		        v.view_count, v.created_at, v.published_at, v.duration_seconds, v.width, v.height,
		        v.has_dash, v.has_subtitles,
		        v.owner_id, u.display_name
		 FROM videos v
		 JOIN users u ON u.id = v.owner_id
		 WHERE v.id = $1 AND v.deleted_at IS NULL`,
		videoID,
	).Scan(&id, &title, &description, &visibility, &status,
		&viewCount, &createdAt, &publishedAt, &durationSeconds, &width, &height,
		&hasDash, &hasSubtitles,
		&ownerID, &displayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "video not found")
			return
		}
		h.logger.Error("get video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Private videos are only visible to their owner.
	if visibility == "private" || visibility == "removed" {
		callerID := middleware.GetUserID(r)
		if callerID != ownerID {
			writeError(w, http.StatusNotFound, "video not found")
			return
		}
	}

	// Fetch ready renditions.
	rows, err := h.db.Query(r.Context(),
		`SELECT name, width, height, hls_manifest_object_key
		 FROM video_renditions
		 WHERE video_id = $1 AND status = 'ready'
		 ORDER BY height DESC`,
		videoID,
	)
	if err != nil {
		h.logger.Error("get video renditions", "error", err)
	}

	var renditions []RenditionInfo
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, hlsKey string
			var rw, rh int
			if err := rows.Scan(&name, &rw, &rh, &hlsKey); err != nil {
				h.logger.Error("scan rendition row", "error", err)
				continue
			}
			url, _ := h.storage.PresignGet(r.Context(), "tpt-media", hlsKey, 1*time.Hour)
			renditions = append(renditions, RenditionInfo{Name: name, Width: rw, Height: rh, URL: url})
		}
	}

	var thumbnailURL string
	if status == "ready" {
		thumbnailKey := "hls/" + videoID + "/thumbnail.jpg"
		thumbnailURL, _ = h.storage.PresignGet(r.Context(), "tpt-media", thumbnailKey, 24*time.Hour)
	}

	var hlsManifestURL string
	if status == "ready" {
		masterKey := "hls/" + videoID + "/master.m3u8"
		hlsManifestURL, _ = h.storage.PresignGet(r.Context(), "tpt-media", masterKey, 1*time.Hour)
	}

	var dashManifestURL string
	if status == "ready" && hasDash {
		dashKey := "dash/" + videoID + "/manifest.mpd"
		dashManifestURL, _ = h.storage.PresignGet(r.Context(), "tpt-media", dashKey, 1*time.Hour)
	}

	var subtitleTracks []SubtitleTrackInfo
	if status == "ready" && hasSubtitles {
		vttKey := "subtitles/" + videoID + "/default.vtt"
		vttURL, err := h.storage.PresignGet(r.Context(), "tpt-media", vttKey, 1*time.Hour)
		if err == nil {
			subtitleTracks = []SubtitleTrackInfo{
				{Language: "en", Label: "English", URL: vttURL},
			}
		}
	}

	var pubAt *string
	if publishedAt != nil {
		s := publishedAt.UTC().Format(time.RFC3339)
		pubAt = &s
	}

	createdAtStr := createdAt.UTC().Format(time.RFC3339)

	resp := VideoResponse{
		ID:              id,
		Title:           title,
		Description:     description,
		Visibility:      visibility,
		Status:          status,
		DurationSeconds: durationSeconds,
		Width:           width,
		Height:          height,
		ViewCount:       viewCount,
		CreatedAt:       createdAtStr,
		PublishedAt:     pubAt,
		ThumbnailURL:    thumbnailURL,
		HLSManifestURL:  hlsManifestURL,
		DASHManifestURL: dashManifestURL,
		Renditions:      renditions,
		SubtitleTracks:  subtitleTracks,
		Owner:           &OwnerInfo{ID: ownerID, DisplayName: displayName},
	}

	// Increment view count async with Redis-based dedup (1 view per IP per video per hour).
	clientIP := r.RemoteAddr
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		clientIP = strings.SplitN(xff, ",", 2)[0]
	}
	dedupeKey := "view:dedup:" + videoID + ":" + strings.TrimSpace(clientIP)
	go func() {
		ctx := context.Background()
		set, err := h.redis.SetNX(ctx, dedupeKey, 1, time.Hour).Result()
		if err != nil || !set {
			return
		}
		_, _ = h.db.Exec(ctx,
			`UPDATE videos SET view_count = view_count + 1 WHERE id = $1`, videoID)
	}()

	writeJSON(w, http.StatusOK, resp)
}

type VideoListItem struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	DurationSeconds *float64 `json:"duration_seconds,omitempty"`
	ViewCount       int64    `json:"view_count"`
	CreatedAt       string   `json:"created_at"`
	ThumbnailURL    string   `json:"thumbnail_url,omitempty"`
	OwnerName       string   `json:"owner_name"`
}

func (h *VideoHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(),
		`SELECT v.id, v.title, v.status::text, v.duration_seconds, v.view_count, v.created_at, u.display_name
		 FROM videos v
		 JOIN users u ON u.id = v.owner_id
		 WHERE v.visibility = 'public' AND v.status = 'ready' AND v.deleted_at IS NULL
		 ORDER BY v.created_at DESC
		 LIMIT 50`,
	)
	if err != nil {
		h.logger.Error("list videos", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	var videos []VideoListItem
	for rows.Next() {
		var id, title, status, ownerName string
		var viewCount int64
		var createdAt time.Time
		var durationSeconds *float64

		if err := rows.Scan(&id, &title, &status, &durationSeconds, &viewCount, &createdAt, &ownerName); err != nil {
			h.logger.Error("scan video row", "error", err)
			continue
		}

		thumbURL := ""
		if status == "ready" {
			thumbURL, _ = h.storage.PresignGet(r.Context(), "tpt-media", "hls/"+id+"/thumbnail.jpg", 24*time.Hour)
		}

		videos = append(videos, VideoListItem{
			ID:              id,
			Title:           title,
			Status:          status,
			DurationSeconds: durationSeconds,
			ViewCount:       viewCount,
			CreatedAt:       createdAt.UTC().Format(time.RFC3339),
			ThumbnailURL:    thumbURL,
			OwnerName:       ownerName,
		})
	}

	if videos == nil {
		videos = []VideoListItem{}
	}

	writeJSON(w, http.StatusOK, videos)
}

// UpdateVideoRequest holds the fields that can be patched on a video.
type UpdateVideoRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Visibility  *string `json:"visibility"`
}

func (h *VideoHandler) UpdateVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	var req UpdateVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Title == nil && req.Description == nil && req.Visibility == nil {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	if req.Title != nil && strings.TrimSpace(*req.Title) == "" {
		writeError(w, http.StatusBadRequest, "title cannot be empty")
		return
	}

	validVisibilities := map[string]bool{"public": true, "unlisted": true, "private": true}
	if req.Visibility != nil && !validVisibilities[*req.Visibility] {
		writeError(w, http.StatusBadRequest, "visibility must be public, unlisted, or private")
		return
	}

	// Confirm the caller owns the video.
	var ownerID string
	err := h.db.QueryRow(r.Context(),
		`SELECT owner_id FROM videos WHERE id = $1 AND deleted_at IS NULL`, videoID,
	).Scan(&ownerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "video not found")
			return
		}
		h.logger.Error("update video owner check", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if ownerID != callerID {
		writeError(w, http.StatusForbidden, "you do not own this video")
		return
	}

	// Build dynamic SET clause.
	setClauses := []string{"updated_at = NOW()"}
	args := []any{}
	i := 1

	if req.Title != nil {
		setClauses = append(setClauses, "title = $"+itoa(i))
		args = append(args, strings.TrimSpace(*req.Title))
		i++
	}
	if req.Description != nil {
		setClauses = append(setClauses, "description = $"+itoa(i))
		args = append(args, *req.Description)
		i++
	}
	if req.Visibility != nil {
		setClauses = append(setClauses, "visibility = $"+itoa(i)+"::video_visibility")
		args = append(args, *req.Visibility)
		i++
	}

	args = append(args, videoID)
	query := "UPDATE videos SET " + strings.Join(setClauses, ", ") + " WHERE id = $" + itoa(i)

	if _, err := h.db.Exec(r.Context(), query, args...); err != nil {
		h.logger.Error("update video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": videoID})
}

func (h *VideoHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	// Soft-delete: only if caller is the owner.
	tag, err := h.db.Exec(r.Context(),
		`UPDATE videos SET deleted_at = NOW(), visibility = 'removed'::video_visibility
		 WHERE id = $1 AND owner_id = $2 AND deleted_at IS NULL`,
		videoID, callerID,
	)
	if err != nil {
		h.logger.Error("delete video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SignedURLsResponse holds fresh presigned media URLs for a video.
type SignedURLsResponse struct {
	VideoID        string          `json:"video_id"`
	ThumbnailURL   string          `json:"thumbnail_url,omitempty"`
	HLSManifestURL string          `json:"hls_manifest_url,omitempty"`
	Renditions     []RenditionInfo `json:"renditions,omitempty"`
	ExpiresIn      int             `json:"expires_in_seconds"`
}

func (h *VideoHandler) GetSignedURLs(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	var ownerID, status, visibility string
	err := h.db.QueryRow(r.Context(),
		`SELECT owner_id, status::text, visibility::text FROM videos WHERE id = $1 AND deleted_at IS NULL`,
		videoID,
	).Scan(&ownerID, &status, &visibility)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "video not found")
			return
		}
		h.logger.Error("get signed urls", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if (visibility == "private" || visibility == "removed") && callerID != ownerID {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}

	if status != "ready" {
		writeError(w, http.StatusConflict, "video is not ready")
		return
	}

	const ttl = 1 * time.Hour

	thumbnailURL, _ := h.storage.PresignGet(r.Context(), "tpt-media", "hls/"+videoID+"/thumbnail.jpg", 24*time.Hour)
	manifestURL, _ := h.storage.PresignGet(r.Context(), "tpt-media", "hls/"+videoID+"/master.m3u8", ttl)

	rows, err := h.db.Query(r.Context(),
		`SELECT name, width, height, hls_manifest_object_key
		 FROM video_renditions WHERE video_id = $1 AND status = 'ready' ORDER BY height DESC`,
		videoID,
	)
	var renditions []RenditionInfo
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name, hlsKey string
			var rw, rh int
			if err := rows.Scan(&name, &rw, &rh, &hlsKey); err != nil {
				continue
			}
			url, _ := h.storage.PresignGet(r.Context(), "tpt-media", hlsKey, ttl)
			renditions = append(renditions, RenditionInfo{Name: name, Width: rw, Height: rh, URL: url})
		}
	}

	writeJSON(w, http.StatusOK, SignedURLsResponse{
		VideoID:        videoID,
		ThumbnailURL:   thumbnailURL,
		HLSManifestURL: manifestURL,
		Renditions:     renditions,
		ExpiresIn:      int(ttl.Seconds()),
	})
}

func (h *VideoHandler) RelatedVideos(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	// Return other public ready videos from the same owner, then fill with recent public videos.
	rows, err := h.db.Query(r.Context(),
		`SELECT v.id, v.title, v.status::text, v.duration_seconds, v.view_count, v.created_at, u.display_name
		 FROM videos v
		 JOIN users u ON u.id = v.owner_id
		 WHERE v.id != $1
		   AND v.visibility = 'public'
		   AND v.status = 'ready'
		   AND v.deleted_at IS NULL
		   AND v.owner_id = (SELECT owner_id FROM videos WHERE id = $1)
		 ORDER BY v.created_at DESC
		 LIMIT 8`,
		videoID,
	)
	if err != nil {
		h.logger.Error("related videos", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	var videos []VideoListItem
	for rows.Next() {
		var id, title, status, ownerName string
		var viewCount int64
		var createdAt time.Time
		var durationSeconds *float64

		if err := rows.Scan(&id, &title, &status, &durationSeconds, &viewCount, &createdAt, &ownerName); err != nil {
			h.logger.Error("scan related video row", "error", err)
			continue
		}

		thumbURL, _ := h.storage.PresignGet(r.Context(), "tpt-media", "hls/"+id+"/thumbnail.jpg", 24*time.Hour)

		videos = append(videos, VideoListItem{
			ID:              id,
			Title:           title,
			Status:          status,
			DurationSeconds: durationSeconds,
			ViewCount:       viewCount,
			CreatedAt:       createdAt.UTC().Format(time.RFC3339),
			ThumbnailURL:    thumbURL,
			OwnerName:       ownerName,
		})
	}

	if videos == nil {
		videos = []VideoListItem{}
	}

	writeJSON(w, http.StatusOK, videos)
}

// itoa converts a small integer to its decimal string representation.
func itoa(n int) string {
	const digits = "0123456789"
	if n < 10 {
		return string(digits[n])
	}
	b := make([]byte, 0, 3)
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}
