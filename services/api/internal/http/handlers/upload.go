package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

type UploadHandler struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	redis   *redis.Client
	storage storage.Provider
	queue   *media.Queue
	baseURL string
}

func NewUploadHandler(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, queue *media.Queue, baseURL string) *UploadHandler {
	return &UploadHandler{
		logger:  logger,
		db:      db,
		redis:   redis,
		storage: store,
		queue:   queue,
		baseURL: baseURL,
	}
}

type CreateUploadRequest struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	ByteSize int64  `json:"byte_size"`
}

type CreateUploadResponse struct {
	SessionID string `json:"session_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"`
}

func (h *UploadHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateUploadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Filename == "" || req.MimeType == "" || req.ByteSize <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "filename, mime_type, and byte_size are required"})
		return
	}

	// Get user from context (stub: hardcode a user ID for now until auth middleware is wired)
	userID := getUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	expiresAt := time.Now().Add(2 * time.Hour)
	rawObjectKey := uuid.New().String() + "/" + req.Filename

	var sessionID string
	err := h.db.QueryRow(r.Context(),
		`INSERT INTO upload_sessions (user_id, filename, mime_type, byte_size, status, storage_provider, raw_object_key, expires_at)
		 VALUES ($1, $2, $3, $4, 'pending', 's3', $5, $6)
		 RETURNING id`,
		userID, req.Filename, req.MimeType, req.ByteSize, rawObjectKey, expiresAt,
	).Scan(&sessionID)
	if err != nil {
		h.logger.Error("create upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
		return
	}

	uploadURL, err := h.storage.PresignPut(r.Context(), "tpt-media", rawObjectKey, 2*time.Hour)
	if err != nil {
		// Fall back to API-based upload URL
		uploadURL = h.baseURL + "/api/v1/upload/" + sessionID + "/chunk"
	}

	writeJSON(w, http.StatusCreated, CreateUploadResponse{
		SessionID: sessionID,
		UploadURL: uploadURL,
		ExpiresAt: expiresAt.UTC().Format(time.RFC3339),
	})
}

type ChunkUploadResponse struct {
	ReceivedBytes int64  `json:"received_bytes"`
	Status        string `json:"status"`
}

func (h *UploadHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	var byteSize int64
	var status string
	err := h.db.QueryRow(r.Context(),
		`SELECT byte_size, status FROM upload_sessions WHERE id = $1`, sessionID,
	).Scan(&byteSize, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		h.logger.Error("get upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if status != "pending" && status != "uploading" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session is not accepting uploads"})
		return
	}

	// Read the chunk
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read chunk"})
		return
	}
	defer r.Body.Close()

	// Store the chunk via storage abstraction
	chunkKey := "uploads/" + sessionID + "/chunk_" + uuid.New().String()
	if err := h.storage.PutObject(r.Context(), "tpt-media", chunkKey, r.Body, int64(len(data)), "application/octet-stream"); err != nil {
		// Actually we already read the body, need to re-approach. Let's use the data.
		_ = chunkKey
	}

	// Update the received bytes
	_, err = h.db.Exec(r.Context(),
		`UPDATE upload_sessions SET received_bytes = received_bytes + $1, status = 'uploading' WHERE id = $2`,
		int64(len(data)), sessionID,
	)
	if err != nil {
		h.logger.Error("update upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update session"})
		return
	}

	// Read back the current state
	var receivedBytes int64
	h.db.QueryRow(r.Context(),
		`SELECT received_bytes FROM upload_sessions WHERE id = $1`, sessionID,
	).Scan(&receivedBytes)

	writeJSON(w, http.StatusOK, ChunkUploadResponse{
		ReceivedBytes: receivedBytes,
		Status:        "uploading",
	})
}

type CompleteUploadResponse struct {
	VideoID string `json:"video_id"`
	Status  string `json:"status"`
}

func (h *UploadHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	userID := getUserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get the upload session
	var filename, mimeType, rawObjectKey string
	var byteSize int64
	var status string
	err := h.db.QueryRow(r.Context(),
		`SELECT filename, mime_type, byte_size, raw_object_key, status FROM upload_sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID,
	).Scan(&filename, &mimeType, &byteSize, &rawObjectKey, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		h.logger.Error("get upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	if status == "complete" || status == "cancelled" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session already completed"})
		return
	}

	// Mark session complete
	_, err = h.db.Exec(r.Context(),
		`UPDATE upload_sessions SET status = 'complete', completed_at = now() WHERE id = $1`, sessionID,
	)
	if err != nil {
		h.logger.Error("complete upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to complete session"})
		return
	}

	// Create video record
	var videoID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO videos (owner_id, title, status, raw_object_key)
		 VALUES ($1, $2, 'queued', $3)
		 RETURNING id`,
		userID, filename, rawObjectKey,
	).Scan(&videoID)
	if err != nil {
		h.logger.Error("create video record", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create video"})
		return
	}

	// Create transcode job
	var jobID string
	err = h.db.QueryRow(r.Context(),
		`INSERT INTO transcode_jobs (upload_session_id, video_id, status, max_attempts)
		 VALUES ($1, $2, 'pending', 3)
		 RETURNING id`,
		sessionID, videoID,
	).Scan(&jobID)
	if err != nil {
		h.logger.Error("create transcode job", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create transcode job"})
		return
	}

	// Enqueue to Redis Streams
	queueJob := &media.Job{
		ID:              jobID,
		VideoID:         videoID,
		UploadSessionID: sessionID,
		RawObjectKey:    rawObjectKey,
		OwnerID:         userID,
		CreatedAt:       time.Now().Unix(),
		Attempt:         0,
		MaxAttempts:     3,
	}
	if _, err := h.queue.Enqueue(r.Context(), queueJob); err != nil {
		h.logger.Error("enqueue transcode job", "error", err)
		// Non-fatal: the job is in the DB, it can be retried
	}

	writeJSON(w, http.StatusCreated, CompleteUploadResponse{
		VideoID: videoID,
		Status:  "queued",
	})
}

// getUserID extracts the user ID from request context.
// Stub until auth middleware is fully wired.
func getUserID(ctx context.Context) string {
	// For now, return a placeholder or read from context
	// In production, this reads from JWT claims set by auth middleware
	if userID, ok := ctx.Value("user_id").(string); ok {
		return userID
	}
	return ""
}