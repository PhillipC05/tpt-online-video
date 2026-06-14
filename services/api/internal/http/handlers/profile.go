package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

// ProfileHandler handles channel/profile HTTP requests.
type ProfileHandler struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	storage storage.Provider
}

// NewProfileHandler creates a new profile handler.
func NewProfileHandler(logger *slog.Logger, db *pgxpool.Pool, store storage.Provider) *ProfileHandler {
	return &ProfileHandler{
		logger:  logger,
		db:      db,
		storage: store,
	}
}

// ---------- Response Types ----------

type ChannelResponse struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	Bio         *string `json:"bio,omitempty"`
	AvatarURL   string  `json:"avatar_url,omitempty"`
	BannerURL   string  `json:"banner_url,omitempty"`
	VideoCount  int     `json:"video_count"`
	CreatedAt   string  `json:"created_at"`
}

// ---------- Handlers ----------

// GET /api/v1/channels/{userID}
func (h *ProfileHandler) GetChannel(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	var id, displayName string
	var bio, avatarKey, bannerKey *string
	var createdAt time.Time
	var videoCount int

	err := h.db.QueryRow(r.Context(),
		`SELECT u.id, u.display_name, u.bio, u.avatar_key, u.banner_key, u.created_at,
		        COALESCE((SELECT COUNT(*) FROM videos v WHERE v.owner_id = u.id AND v.visibility = 'public' AND v.status = 'ready' AND v.deleted_at IS NULL), 0) AS video_count
		 FROM users u
		 WHERE u.id = $1 AND u.deleted_at IS NULL`,
		userID,
	).Scan(&id, &displayName, &bio, &avatarKey, &bannerKey, &createdAt, &videoCount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "channel not found")
			return
		}
		h.logger.Error("get channel", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	resp := ChannelResponse{
		ID:          id,
		DisplayName: displayName,
		Bio:         bio,
		VideoCount:  videoCount,
		CreatedAt:   createdAt.UTC().Format(time.RFC3339),
	}

	if avatarKey != nil && *avatarKey != "" {
		url, err := h.storage.PresignGet(r.Context(), "tpt-media", *avatarKey, 24*time.Hour)
		if err == nil {
			resp.AvatarURL = url
		}
	}
	if bannerKey != nil && *bannerKey != "" {
		url, err := h.storage.PresignGet(r.Context(), "tpt-media", *bannerKey, 24*time.Hour)
		if err == nil {
			resp.BannerURL = url
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/channels/{userID}/videos
func (h *ProfileHandler) ListChannelVideos(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	callerID := middleware.GetUserID(r)
	isOwner := callerID == userID

	var (
		pgrows pgx.Rows
		err    error
	)

	// If the caller is the owner, show all their public+unlisted+private ready videos.
	// For non-owners, show only public ready videos.
	if isOwner {
		pgrows, err = h.db.Query(r.Context(),
			`SELECT v.id, v.title, v.status::text, v.visibility::text, v.duration_seconds, v.view_count, v.created_at
			 FROM videos v
			 WHERE v.owner_id = $1 AND v.status = 'ready' AND v.deleted_at IS NULL
			 ORDER BY v.created_at DESC
			 LIMIT 50`,
			userID,
		)
	} else {
		pgrows, err = h.db.Query(r.Context(),
			`SELECT v.id, v.title, v.status::text, v.visibility::text, v.duration_seconds, v.view_count, v.created_at
			 FROM videos v
			 WHERE v.owner_id = $1 AND v.visibility = 'public' AND v.status = 'ready' AND v.deleted_at IS NULL
			 ORDER BY v.created_at DESC
			 LIMIT 50`,
			userID,
		)
	}

	if err != nil {
		h.logger.Error("list channel videos", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer pgrows.Close()

	type ChannelVideoItem struct {
		ID              string   `json:"id"`
		Title           string   `json:"title"`
		Status          string   `json:"status"`
		Visibility      string   `json:"visibility"`
		DurationSeconds *float64 `json:"duration_seconds,omitempty"`
		ViewCount       int64    `json:"view_count"`
		CreatedAt       string   `json:"created_at"`
		ThumbnailURL    string   `json:"thumbnail_url,omitempty"`
	}

	var videos []ChannelVideoItem
	for pgrows.Next() {
		var id, title, status, visibility string
		var viewCount int64
		var createdAt time.Time
		var durationSeconds *float64

		if err := pgrows.Scan(&id, &title, &status, &visibility, &durationSeconds, &viewCount, &createdAt); err != nil {
			h.logger.Error("scan channel video row", "error", err)
			continue
		}

		thumbURL := ""
		if status == "ready" {
			thumbURL, _ = h.storage.PresignGet(r.Context(), "tpt-media", "hls/"+id+"/thumbnail.jpg", 24*time.Hour)
		}

		videos = append(videos, ChannelVideoItem{
			ID:              id,
			Title:           title,
			Status:          status,
			Visibility:      visibility,
			DurationSeconds: durationSeconds,
			ViewCount:       viewCount,
			CreatedAt:       createdAt.UTC().Format(time.RFC3339),
			ThumbnailURL:    thumbURL,
		})
	}

	if videos == nil {
		videos = []ChannelVideoItem{}
	}

	writeJSON(w, http.StatusOK, videos)
}

// GET /api/v1/channels/{userID}/live
func (h *ProfileHandler) ListChannelLiveStreams(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, title, status::text, started_at, created_at
		 FROM live_streams
		 WHERE owner_id = $1
		 ORDER BY created_at DESC
		 LIMIT 20`,
		userID,
	)
	if err != nil {
		h.logger.Error("list channel live streams", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	type ChannelLiveItem struct {
		ID        string  `json:"id"`
		Title     string  `json:"title"`
		Status    string  `json:"status"`
		StartedAt *string `json:"started_at,omitempty"`
		CreatedAt string  `json:"created_at"`
	}

	var streams []ChannelLiveItem
	for rows.Next() {
		var id, title, status string
		var createdAt time.Time
		var startedAt *time.Time

		if err := rows.Scan(&id, &title, &status, &startedAt, &createdAt); err != nil {
			h.logger.Error("scan live stream row", "error", err)
			continue
		}

		var startedAtStr *string
		if startedAt != nil {
			s := startedAt.UTC().Format(time.RFC3339)
			startedAtStr = &s
		}

		streams = append(streams, ChannelLiveItem{
			ID:        id,
			Title:     title,
			Status:    status,
			StartedAt: startedAtStr,
			CreatedAt: createdAt.UTC().Format(time.RFC3339),
		})
	}

	if streams == nil {
		streams = []ChannelLiveItem{}
	}

	writeJSON(w, http.StatusOK, streams)
}

// ---------- Profile Update ----------

type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
}

// PATCH /api/v1/auth/me/profile
func (h *ProfileHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DisplayName == nil && req.Bio == nil {
		writeError(w, http.StatusBadRequest, "no fields to update")
		return
	}

	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	i := 1

	if req.DisplayName != nil {
		clean, err := validateDisplayName(*req.DisplayName)
		if err != nil {
			writeError(w, http.StatusBadRequest, "display_name: "+err.Error())
			return
		}
		setClauses = append(setClauses, "display_name = $"+itoa(i))
		args = append(args, clean)
		i++
	}
	if req.Bio != nil {
		clean, err := validateBio(*req.Bio)
		if err != nil {
			writeError(w, http.StatusBadRequest, "bio: "+err.Error())
			return
		}
		setClauses = append(setClauses, "bio = $"+itoa(i))
		args = append(args, clean)
		i++
	}

	args = append(args, userID)
	query := "UPDATE users SET " + joinStrings(setClauses, ", ") + " WHERE id = $" + itoa(i)

	if _, err := h.db.Exec(r.Context(), query, args...); err != nil {
		h.logger.Error("update profile", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Return updated user info
	var id, displayName string
	var bio, avatarKey, bannerKey *string
	err := h.db.QueryRow(r.Context(),
		`SELECT id, display_name, bio, avatar_key, banner_key FROM users WHERE id = $1`,
		userID,
	).Scan(&id, &displayName, &bio, &avatarKey, &bannerKey)
	if err != nil {
		h.logger.Error("get updated profile", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":           id,
		"display_name": displayName,
		"bio":          bio,
		"avatar_key":   avatarKey,
		"banner_key":   bannerKey,
	})
}

// ---------- Avatar / Banner Upload ----------

type UploadImageResponse struct {
	URL string `json:"url"`
	Key string `json:"key"`
}

// POST /api/v1/auth/me/avatar
func (h *ProfileHandler) UploadAvatar(w http.ResponseWriter, r *http.Request) {
	h.uploadChannelImage(w, r, "avatar")
}

// POST /api/v1/auth/me/banner
func (h *ProfileHandler) UploadBanner(w http.ResponseWriter, r *http.Request) {
	h.uploadChannelImage(w, r, "banner")
}

func (h *ProfileHandler) uploadChannelImage(w http.ResponseWriter, r *http.Request, imageType string) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Parse multipart form (max 5 MB for avatars, 10 MB for banners)
	maxSize := int64(5 << 20) // 5 MB
	if imageType == "banner" {
		maxSize = 10 << 20 // 10 MB
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	if err := r.ParseMultipartForm(maxSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field is required")
		return
	}
	defer file.Close()

	// Validate content type
	contentType := header.Header.Get("Content-Type")
	validTypes := map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
		"image/webp": true,
		"image/gif":  true,
	}
	if !validTypes[contentType] {
		writeError(w, http.StatusBadRequest, "invalid image type. Accepted: jpeg, png, webp, gif")
		return
	}

	// Generate key
	ext := ".jpg"
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	key := "channels/" + userID + "/" + imageType + ext

	// Upload directly to storage
	if err := h.storage.PutObject(r.Context(), "tpt-media", key, file, header.Size, contentType); err != nil {
		h.logger.Error("upload channel image", "error", err)
		writeError(w, http.StatusInternalServerError, "upload failed")
		return
	}

	// Update user record with the key
	var column string
	if imageType == "avatar" {
		column = "avatar_key"
	} else {
		column = "banner_key"
	}

	if _, err := h.db.Exec(r.Context(),
		"UPDATE users SET "+column+" = $1, updated_at = NOW() WHERE id = $2",
		key, userID,
	); err != nil {
		h.logger.Error("update user "+column, "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Generate presigned URL for immediate use
	presignedURL, err := h.storage.PresignGet(r.Context(), "tpt-media", key, 24*time.Hour)
	if err != nil {
		h.logger.Error("generate presigned URL", "error", err)
	}

	writeJSON(w, http.StatusOK, UploadImageResponse{
		URL: presignedURL,
		Key: key,
	})
}

// joinStrings joins strings with a separator (replacement for strings.Join to avoid import in this file).
func joinStrings(elems []string, sep string) string {
	if len(elems) == 0 {
		return ""
	}
	result := elems[0]
	for _, e := range elems[1:] {
		result += sep + e
	}
	return result
}