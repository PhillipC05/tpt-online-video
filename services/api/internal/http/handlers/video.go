package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/storage"
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
	ID              string            `json:"id"`
	Title           string            `json:"title"`
	Description     string            `json:"description,omitempty"`
	Visibility      string            `json:"visibility"`
	Status          string            `json:"status"`
	DurationSeconds *float64          `json:"duration_seconds,omitempty"`
	Width           *int              `json:"width,omitempty"`
	Height          *int              `json:"height,omitempty"`
	ViewCount       int64             `json:"view_count"`
	CreatedAt       string            `json:"created_at"`
	PublishedAt     *string           `json:"published_at,omitempty"`
	ThumbnailURL    string            `json:"thumbnail_url,omitempty"`
	HLSManifestURL  string            `json:"hls_manifest_url,omitempty"`
	Renditions      []RenditionInfo   `json:"renditions,omitempty"`
	Owner           *OwnerInfo        `json:"owner,omitempty"`
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

	err := h.db.QueryRow(r.Context(),
		`SELECT v.id, v.title, COALESCE(v.description, ''), v.visibility::text, v.status::text,
		        v.view_count, v.created_at, v.published_at, v.duration_seconds, v.width, v.height,
		        v.owner_id, u.display_name
		 FROM videos v
		 JOIN users u ON u.id = v.owner_id
		 WHERE v.id = $1 AND v.deleted_at IS NULL`,
		videoID,
	).Scan(&id, &title, &description, &visibility, &status,
		&viewCount, &createdAt, &publishedAt, &durationSeconds, &width, &height,
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

	// Fetch renditions
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
			var w, h int
			if err := rows.Scan(&name, &w, &h, &hlsKey); err != nil {
				h.logger.Error("scan rendition row", "error", err)
				continue
			}
			url, _ := h.storage.PresignGet(r.Context(), "tpt-media", hlsKey, 1*time.Hour)
			renditions = append(renditions, RenditionInfo{Name: name, Width: w, Height: h, URL: url})
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
		Renditions:      renditions,
		Owner:           &OwnerInfo{ID: ownerID, DisplayName: displayName},
	}

	// Increment view count async
	go func() {
		_, _ = h.db.Exec(context.Background(),
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