package handlers

import (
	"bytes"
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
	"github.com/tpt-online-video/packages/moderation"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

const (
	// DefaultUploadExpiration is how long an upload session remains valid.
	DefaultUploadExpiration = 2 * time.Hour

	// DefaultMaxUploadBytes is the maximum allowed file size (10 GB).
	DefaultMaxUploadBytes int64 = 10 * 1024 * 1024 * 1024
)

type UploadHandler struct {
	logger     *slog.Logger
	db         *pgxpool.Pool
	redis      *redis.Client
	storage    storage.Provider
	queue      *media.Queue
	baseURL    string
	scanner    moderation.Scanner
	maxBytes   int64
}

func NewUploadHandler(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, queue *media.Queue, baseURL string) *UploadHandler {
	return &UploadHandler{
		logger:   logger,
		db:       db,
		redis:    redis,
		storage:  store,
		queue:    queue,
		baseURL:  baseURL,
		scanner:  moderation.NewNopScanner(),
		maxBytes: DefaultMaxUploadBytes,
	}
}

// WithScanner sets a malware scanner on the handler (for production use).
func (h *UploadHandler) WithScanner(s moderation.Scanner) *UploadHandler {
	h.scanner = s
	return h
}

// WithMaxBytes sets a custom maximum file size limit.
func (h *UploadHandler) WithMaxBytes(max int64) *UploadHandler {
	h.maxBytes = max
	return h
}

type CreateUploadRequest struct {
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
	ByteSize int64  `json:"byte_size"`
}

type CreateUploadResponse struct {
	SessionID       string `json:"session_id"`
	UploadURL       string `json:"upload_url"`
	UploadMethod    string `json:"upload_method"` // "presigned" or "chunked"
	ExpiresAt       string `json:"expires_at"`
	AllowedMIMETypes []string `json:"allowed_mime_types,omitempty"`
	MaxBytes        int64  `json:"max_bytes"`
}

type UploadSessionStatus struct {
	SessionID     string `json:"session_id"`
	Filename      string `json:"filename"`
	MimeType      string `json:"mime_type"`
	ByteSize      int64  `json:"byte_size"`
	ReceivedBytes int64  `json:"received_bytes"`
	Status        string `json:"status"`
	ExpiresAt     string `json:"expires_at"`
	CreatedAt     string `json:"created_at"`
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

	// --- Validation ---

	// File type validation
	if err := moderation.ValidateFileType(req.Filename, req.MimeType); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// File size validation
	if err := moderation.ValidateFileSize(req.ByteSize, h.maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	// Get user from context
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Build allowed MIME type list for response
	allowedMIMEs := make([]string, 0, len(moderation.AllowedVideoMIMEs))
	for mime := range moderation.AllowedVideoMIMEs {
		allowedMIMEs = append(allowedMIMEs, mime)
	}

	expiresAt := time.Now().Add(DefaultUploadExpiration)
	rawObjectKey := uuid.New().String() + "/" + req.Filename

	// Determine whether to use presigned or chunked upload
	usePresigned := req.ByteSize < 100*1024*1024 // < 100MB, use presigned
	uploadMethod := "chunked"
	if usePresigned {
		uploadMethod = "presigned"
	}

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

	var uploadURL string
	if usePresigned {
		uploadURL, err = h.storage.PresignPut(r.Context(), "tpt-media", rawObjectKey, DefaultUploadExpiration)
		if err != nil {
			// Fall back to API-based upload URL
			uploadURL = h.baseURL + "/api/v1/upload/" + sessionID + "/chunk"
			uploadMethod = "chunked"
		}
	} else {
		uploadURL = h.baseURL + "/api/v1/upload/" + sessionID + "/chunk"
	}

	writeJSON(w, http.StatusCreated, CreateUploadResponse{
		SessionID:       sessionID,
		UploadURL:       uploadURL,
		UploadMethod:    uploadMethod,
		ExpiresAt:       expiresAt.UTC().Format(time.RFC3339),
		AllowedMIMETypes: allowedMIMEs,
		MaxBytes:        h.maxBytes,
	})
}

type ChunkUploadResponse struct {
	ReceivedBytes int64  `json:"received_bytes"`
	Status        string `json:"status"`
}

func (h *UploadHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	userID := middleware.GetUserID(r)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var byteSize int64
	var status string
	err := h.db.QueryRow(r.Context(),
		`SELECT byte_size, status FROM upload_sessions WHERE id = $1 AND user_id = $2`, sessionID, userID,
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

	// Read the chunk, capped at the declared session size + 1 byte to detect over-sized bodies.
	data, err := io.ReadAll(io.LimitReader(r.Body, byteSize+1))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read chunk"})
		return
	}
	defer r.Body.Close()
	if int64(len(data)) > byteSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "chunk exceeds declared file size"})
		return
	}

	// Scan chunk for malware before storing
	scanResult, scanErr := h.scanner.Scan(r.Context(), sessionID, bytes.NewReader(data))
	if scanErr != nil {
		h.logger.Error("scan upload chunk", "error", scanErr, "session", sessionID)
	} else if scanResult.Infected {
		h.db.Exec(r.Context(), `UPDATE upload_sessions SET status = 'failed', updated_at = now() WHERE id = $1`, sessionID)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "file rejected: " + scanResult.Threat})
		return
	}

	// Store the chunk via storage abstraction
	chunkKey := "uploads/" + sessionID + "/chunk_" + uuid.New().String()
	if err := h.storage.PutObject(r.Context(), "tpt-media", chunkKey, bytes.NewReader(data), int64(len(data)), "application/octet-stream"); err != nil {
		h.logger.Error("store upload chunk", "error", err, "session", sessionID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store chunk"})
		return
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

	userID := middleware.GetUserID(r)
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

// CancelUpload cancels an in-progress upload session and cleans up any stored data.
func (h *UploadHandler) CancelUpload(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	userID := middleware.GetUserID(r)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Get session and verify ownership
	var status, rawObjectKey string
	err := h.db.QueryRow(r.Context(),
		`SELECT status, raw_object_key FROM upload_sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID,
	).Scan(&status, &rawObjectKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		h.logger.Error("get upload session for cancel", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	if status == "complete" || status == "cancelled" || status == "expired" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session already ended"})
		return
	}

	// Mark session as cancelled
	_, err = h.db.Exec(r.Context(),
		`UPDATE upload_sessions SET status = 'cancelled', updated_at = now() WHERE id = $1`, sessionID,
	)
	if err != nil {
		h.logger.Error("cancel upload session", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel session"})
		return
	}

	// Clean up uploaded data from storage if raw object key exists
	if rawObjectKey != "" {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := h.storage.DeleteObject(ctx, "tpt-media", rawObjectKey); err != nil {
				h.logger.Error("cleanup cancelled upload object", "error", err, "key", rawObjectKey)
			}
		}()
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "cancelled",
		"session_id": sessionID,
	})
}

// GetUploadStatus returns the current status of an upload session.
func (h *UploadHandler) GetUploadStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionID")

	userID := middleware.GetUserID(r)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var filename, mimeType, status string
	var byteSize, receivedBytes int64
	var expiresAt, createdAt time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT filename, mime_type, byte_size, received_bytes, status, expires_at, created_at
		 FROM upload_sessions WHERE id = $1 AND user_id = $2`,
		sessionID, userID,
	).Scan(&filename, &mimeType, &byteSize, &receivedBytes, &status, &expiresAt, &createdAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "session not found"})
			return
		}
		h.logger.Error("get upload session status", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}

	writeJSON(w, http.StatusOK, UploadSessionStatus{
		SessionID:     sessionID,
		Filename:      filename,
		MimeType:      mimeType,
		ByteSize:      byteSize,
		ReceivedBytes: receivedBytes,
		Status:        status,
		ExpiresAt:     expiresAt.UTC().Format(time.RFC3339),
		CreatedAt:     createdAt.UTC().Format(time.RFC3339),
	})
}

// ListUploadSessions returns all upload sessions for the authenticated user.
func (h *UploadHandler) ListUploadSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	rows, err := h.db.Query(r.Context(),
		`SELECT id, filename, mime_type, byte_size, received_bytes, status, expires_at, created_at
		 FROM upload_sessions
		 WHERE user_id = $1
		 ORDER BY created_at DESC
		 LIMIT 50`,
		userID,
	)
	if err != nil {
		h.logger.Error("list upload sessions", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "database error"})
		return
	}
	defer rows.Close()

	var sessions []UploadSessionStatus
	for rows.Next() {
		var sessionID, filename, mimeType, status string
		var byteSize, receivedBytes int64
		var expiresAt, createdAt time.Time
		if err := rows.Scan(&sessionID, &filename, &mimeType, &byteSize, &receivedBytes, &status, &expiresAt, &createdAt); err != nil {
			h.logger.Error("scan upload session row", "error", err)
			continue
		}
		sessions = append(sessions, UploadSessionStatus{
			SessionID:     sessionID,
			Filename:      filename,
			MimeType:      mimeType,
			ByteSize:      byteSize,
			ReceivedBytes: receivedBytes,
			Status:        status,
			ExpiresAt:     expiresAt.UTC().Format(time.RFC3339),
			CreatedAt:     createdAt.UTC().Format(time.RFC3339),
		})
	}

	if sessions == nil {
		sessions = []UploadSessionStatus{}
	}

	writeJSON(w, http.StatusOK, sessions)
}

