package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/search"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
	svclive "github.com/tpt-online-video/services/api/internal/live"
)

// AdminHandler handles admin-only management endpoints.
type AdminHandler struct {
	logger          *slog.Logger
	db              *pgxpool.Pool
	redis           *redis.Client
	queue           *media.Queue
	storageProvider storage.Provider
	searchProvider  search.Provider
	chatHub         *svclive.ChatHub // optional; nil if live streaming not configured
}

func NewAdminHandler(
	logger *slog.Logger,
	db *pgxpool.Pool,
	rdb *redis.Client,
	queue *media.Queue,
	store storage.Provider,
	srch search.Provider,
) *AdminHandler {
	return &AdminHandler{
		logger:          logger,
		db:              db,
		redis:           rdb,
		queue:           queue,
		storageProvider: store,
		searchProvider:  srch,
	}
}

// WithChatHub attaches the chat hub so SystemStatus can report live viewer counts.
func (h *AdminHandler) WithChatHub(hub *svclive.ChatHub) *AdminHandler {
	h.chatHub = hub
	return h
}

// ─── User Management ──────────────────────────────────────────────────────────

type AdminUser struct {
	ID          string   `json:"id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	Roles       []string `json:"roles"`
	CreatedAt   string   `json:"created_at"`
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	status := q.Get("status")
	limitStr := q.Get("limit")
	offsetStr := q.Get("offset")

	limit := 50
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	args := []any{}
	where := "WHERE u.deleted_at IS NULL"
	idx := 1

	if search != "" {
		where += " AND (u.email ILIKE $" + strconv.Itoa(idx) + " OR u.display_name ILIKE $" + strconv.Itoa(idx) + ")"
		args = append(args, "%"+search+"%")
		idx++
	}
	if status != "" {
		where += " AND u.status = $" + strconv.Itoa(idx)
		args = append(args, status)
		idx++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)

	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), `
		SELECT u.id, u.email, u.display_name, u.status, u.created_at,
		       COALESCE(array_agg(ro.name) FILTER (WHERE ro.name IS NOT NULL), ARRAY[]::TEXT[]) AS roles
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		LEFT JOIN roles ro ON ro.id = ur.role_id
		`+where+`
		GROUP BY u.id
		ORDER BY u.created_at DESC
		LIMIT $`+strconv.Itoa(idx)+` OFFSET $`+strconv.Itoa(idx+1),
		args...,
	)
	if err != nil {
		h.logger.Error("admin list users", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	users := make([]AdminUser, 0)
	for rows.Next() {
		var u AdminUser
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.Status, &createdAt, &u.Roles); err != nil {
			h.logger.Error("admin list users scan", "error", err)
			continue
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		users = append(users, u)
	}

	var total int
	_ = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM users u `+where,
		countArgs...,
	).Scan(&total)

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"total": total,
	})
}

type UpdateUserRequest struct {
	Status *string `json:"status"`
	Role   *string `json:"role"`
}

func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)
	callerRole := middleware.GetUserRole(r)
	if callerRole != "admin" {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	// Prevent self-modification: an admin cannot demote or ban themselves,
	// which would otherwise create an irrecoverable lockout.
	if userID == callerID {
		writeError(w, http.StatusForbidden, "admins cannot modify their own account via this endpoint")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status != nil {
		validStatuses := map[string]bool{"active": true, "suspended": true, "banned": true}
		if !validStatuses[*req.Status] {
			writeError(w, http.StatusBadRequest, "invalid status: must be active, suspended, or banned")
			return
		}
		_, err := h.db.Exec(r.Context(),
			`UPDATE users SET status = $1 WHERE id = $2 AND deleted_at IS NULL`,
			*req.Status, userID,
		)
		if err != nil {
			h.logger.Error("admin update user status", "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	if req.Role != nil {
		validRoles := map[string]bool{"admin": true, "moderator": true, "user": true}
		if !validRoles[*req.Role] {
			writeError(w, http.StatusBadRequest, "invalid role: must be admin, moderator, or user")
			return
		}
		// Replace all roles with the new single role
		_, err := h.db.Exec(r.Context(),
			`DELETE FROM user_roles WHERE user_id = $1`, userID,
		)
		if err != nil {
			h.logger.Error("admin clear user roles", "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		_, err = h.db.Exec(r.Context(),
			`INSERT INTO user_roles (user_id, role_id)
			 SELECT $1, id FROM roles WHERE name = $2`,
			userID, *req.Role,
		)
		if err != nil {
			h.logger.Error("admin set user role", "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ─── Video Management ─────────────────────────────────────────────────────────

type AdminVideo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	OwnerName   string `json:"owner_name"`
	Status      string `json:"status"`
	Visibility  string `json:"visibility"`
	ViewCount   int64  `json:"view_count"`
	CreatedAt   string `json:"created_at"`
}

func (h *AdminHandler) ListVideos(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statusFilter := q.Get("status")
	visFilter := q.Get("visibility")
	search := strings.TrimSpace(q.Get("q"))
	limitStr := q.Get("limit")
	offsetStr := q.Get("offset")

	limit := 50
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	args := []any{}
	where := "WHERE v.deleted_at IS NULL"
	idx := 1

	if statusFilter != "" {
		where += " AND v.status = $" + strconv.Itoa(idx)
		args = append(args, statusFilter)
		idx++
	}
	if visFilter != "" {
		where += " AND v.visibility = $" + strconv.Itoa(idx)
		args = append(args, visFilter)
		idx++
	}
	if search != "" {
		where += " AND v.title ILIKE $" + strconv.Itoa(idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), `
		SELECT v.id, v.title, u.display_name, v.status, v.visibility, v.view_count, v.created_at
		FROM videos v
		JOIN users u ON u.id = v.owner_id
		`+where+`
		ORDER BY v.created_at DESC
		LIMIT $`+strconv.Itoa(idx)+` OFFSET $`+strconv.Itoa(idx+1),
		args...,
	)
	if err != nil {
		h.logger.Error("admin list videos", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	videos := make([]AdminVideo, 0)
	for rows.Next() {
		var v AdminVideo
		var createdAt time.Time
		if err := rows.Scan(&v.ID, &v.Title, &v.OwnerName, &v.Status, &v.Visibility, &v.ViewCount, &createdAt); err != nil {
			h.logger.Error("admin list videos scan", "error", err)
			continue
		}
		v.CreatedAt = createdAt.Format(time.RFC3339)
		videos = append(videos, v)
	}

	var total int
	_ = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM videos v JOIN users u ON u.id = v.owner_id `+where,
		countArgs...,
	).Scan(&total)

	writeJSON(w, http.StatusOK, map[string]any{
		"videos": videos,
		"total":  total,
	})
}

type AdminUpdateVideoRequest struct {
	Visibility *string `json:"visibility"`
}

func (h *AdminHandler) UpdateVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	var req AdminUpdateVideoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Visibility == nil {
		writeError(w, http.StatusBadRequest, "visibility required")
		return
	}
	valid := map[string]bool{"public": true, "unlisted": true, "private": true, "removed": true}
	if !valid[*req.Visibility] {
		writeError(w, http.StatusBadRequest, "invalid visibility")
		return
	}

	_, err := h.db.Exec(r.Context(),
		`UPDATE videos SET visibility = $1 WHERE id = $2 AND deleted_at IS NULL`,
		*req.Visibility, videoID,
	)
	if err != nil {
		h.logger.Error("admin update video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *AdminHandler) DeleteVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")

	_, err := h.db.Exec(r.Context(),
		`UPDATE videos SET deleted_at = now(), visibility = 'removed' WHERE id = $1 AND deleted_at IS NULL`,
		videoID,
	)
	if err != nil {
		h.logger.Error("admin delete video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ─── Comment Management ───────────────────────────────────────────────────────

type AdminComment struct {
	ID          string `json:"id"`
	Body        string `json:"body"`
	AuthorName  string `json:"author_name"`
	VideoTitle  string `json:"video_title"`
	VideoID     string `json:"video_id"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
}

func (h *AdminHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statusFilter := q.Get("status")
	search := strings.TrimSpace(q.Get("q"))
	limitStr := q.Get("limit")
	offsetStr := q.Get("offset")

	limit := 50
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	args := []any{}
	where := "WHERE c.deleted_at IS NULL"
	idx := 1

	if statusFilter != "" {
		where += " AND c.status = $" + strconv.Itoa(idx)
		args = append(args, statusFilter)
		idx++
	}
	if search != "" {
		where += " AND c.body ILIKE $" + strconv.Itoa(idx)
		args = append(args, "%"+search+"%")
		idx++
	}

	countArgs := make([]any, len(args))
	copy(countArgs, args)
	args = append(args, limit, offset)

	rows, err := h.db.Query(r.Context(), `
		SELECT c.id, c.body, u.display_name, v.title, v.id, c.status, c.created_at
		FROM comments c
		JOIN users u ON u.id = c.user_id
		JOIN videos v ON v.id = c.video_id
		`+where+`
		ORDER BY c.created_at DESC
		LIMIT $`+strconv.Itoa(idx)+` OFFSET $`+strconv.Itoa(idx+1),
		args...,
	)
	if err != nil {
		h.logger.Error("admin list comments", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	comments := make([]AdminComment, 0)
	for rows.Next() {
		var c AdminComment
		var createdAt time.Time
		if err := rows.Scan(&c.ID, &c.Body, &c.AuthorName, &c.VideoTitle, &c.VideoID, &c.Status, &createdAt); err != nil {
			h.logger.Error("admin list comments scan", "error", err)
			continue
		}
		c.CreatedAt = createdAt.Format(time.RFC3339)
		comments = append(comments, c)
	}

	var total int
	_ = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM comments c JOIN users u ON u.id = c.user_id JOIN videos v ON v.id = c.video_id `+where,
		countArgs...,
	).Scan(&total)

	writeJSON(w, http.StatusOK, map[string]any{
		"comments": comments,
		"total":    total,
	})
}

type AdminUpdateCommentRequest struct {
	Status *string `json:"status"`
}

func (h *AdminHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")

	var req AdminUpdateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Status == nil {
		writeError(w, http.StatusBadRequest, "status required")
		return
	}
	valid := map[string]bool{"visible": true, "hidden": true, "deleted": true}
	if !valid[*req.Status] {
		writeError(w, http.StatusBadRequest, "invalid status: visible, hidden, or deleted")
		return
	}

	_, err := h.db.Exec(r.Context(),
		`UPDATE comments SET status = $1 WHERE id = $2 AND deleted_at IS NULL`,
		*req.Status, commentID,
	)
	if err != nil {
		h.logger.Error("admin update comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ─── System Status ────────────────────────────────────────────────────────────

type SystemStatusResponse struct {
	API      string          `json:"api"`
	Postgres string          `json:"postgres"`
	Redis    string          `json:"redis"`
	Storage  string          `json:"storage"`
	Search   string          `json:"search"`
	Queue    *QueueStatus    `json:"queue"`
	Live     *LiveStatus     `json:"live"`
}

type QueueStatus struct {
	StreamLength     int64  `json:"stream_length"`
	UndeliveredCount int64  `json:"undelivered_count"`
	PendingCount     int64  `json:"pending_count"`
	ConsumerCount    int64  `json:"consumer_count"`
	DLQLength        int64  `json:"dlq_length"`
	OldestPendingAge string `json:"oldest_pending_age,omitempty"`
}

type LiveStatus struct {
	ActiveStreams     int64 `json:"active_streams"`
	IdleStreams       int64 `json:"idle_streams"`
	DVREnabledStreams int64 `json:"dvr_enabled_streams"`  // currently-live streams with DVR active
	ActiveViewers    int   `json:"active_viewers"`        // WebSocket chat connections on this instance
	ChatMsgsTotal    int64 `json:"chat_messages_total"`   // chat messages since process start
}

func (h *AdminHandler) SystemStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := SystemStatusResponse{API: "ok"}

	// Postgres
	if err := h.db.Ping(ctx); err != nil {
		resp.Postgres = err.Error()
	} else {
		resp.Postgres = "ok"
	}

	// Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		resp.Redis = err.Error()
	} else {
		resp.Redis = "ok"
	}

	// Storage
	if err := h.storageProvider.Health(ctx); err != nil {
		resp.Storage = err.Error()
	} else {
		resp.Storage = "ok"
	}

	// Search
	if h.searchProvider != nil {
		if err := h.searchProvider.Health(ctx); err != nil {
			resp.Search = err.Error()
		} else {
			resp.Search = "ok"
		}
	} else {
		resp.Search = "not configured"
	}

	// Queue metrics
	if metrics, err := h.queue.Metrics(ctx); err == nil {
		qs := &QueueStatus{
			StreamLength:     metrics.StreamLength,
			UndeliveredCount: metrics.UndeliveredCount,
			PendingCount:     metrics.PendingCount,
			ConsumerCount:    metrics.ConsumerCount,
			DLQLength:        metrics.DLQLength,
		}
		if metrics.OldestPendingAge > 0 {
			qs.OldestPendingAge = metrics.OldestPendingAge.Round(time.Second).String()
		}
		resp.Queue = qs
	} else {
		h.logger.Warn("system status: queue metrics error", "error", err)
		resp.Queue = &QueueStatus{}
	}

	// Live stream counts
	var activeStreams, idleStreams, dvrEnabledStreams int64
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM live_streams WHERE status = 'live'`).Scan(&activeStreams)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM live_streams WHERE status = 'idle'`).Scan(&idleStreams)
	_ = h.db.QueryRow(ctx, `SELECT COUNT(*) FROM live_streams WHERE status = 'live' AND dvr_enabled = true`).Scan(&dvrEnabledStreams)

	liveStatus := &LiveStatus{
		ActiveStreams:     activeStreams,
		IdleStreams:       idleStreams,
		DVREnabledStreams: dvrEnabledStreams,
	}
	if h.chatHub != nil {
		liveStatus.ActiveViewers = h.chatHub.Viewers()
		liveStatus.ChatMsgsTotal = h.chatHub.ChatMsgsTotal()
	}
	resp.Live = liveStatus

	writeJSON(w, http.StatusOK, resp)
}

// ─── Admin Settings ───────────────────────────────────────────────────────────

type AdminSettings struct {
	Storage    StorageSettings    `json:"storage"`
	Search     SearchSettings     `json:"search"`
	Moderation ModerationSettings `json:"moderation"`
	Live       LiveSettings       `json:"live"`
}

type StorageSettings struct {
	Provider string `json:"provider"`
	Note     string `json:"note"`
}

type SearchSettings struct {
	Provider string `json:"provider"`
	Note     string `json:"note"`
}

type ModerationSettings struct {
	AutoModEnabled bool   `json:"auto_mod_enabled"`
	Note           string `json:"note"`
}

type LiveSettings struct {
	DVREnabled          bool   `json:"dvr_enabled"`
	DVRWindowSeconds    int    `json:"dvr_window_seconds"`
	Note                string `json:"note"`
}

func (h *AdminHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	storageProvider := "local"
	if h.storageProvider != nil {
		storageProvider = h.storageProvider.Name()
	}

	searchProvider := "none"
	if h.searchProvider != nil {
		searchProvider = "postgres_fts"
	}

	settings := AdminSettings{
		Storage: StorageSettings{
			Provider: storageProvider,
			Note:     "Configure via STORAGE_PROVIDER environment variable. Supported: local, s3, wasabi.",
		},
		Search: SearchSettings{
			Provider: searchProvider,
			Note:     "Configure via SEARCH_PROVIDER environment variable. Supported: postgres, meilisearch.",
		},
		Moderation: ModerationSettings{
			AutoModEnabled: false,
			Note:           "Auto-moderation configuration is managed via environment variables.",
		},
		Live: LiveSettings{
			DVREnabled:       true,
			DVRWindowSeconds: 900,
			Note:             "Live and DVR settings are configured via MEDIAMTX_* environment variables.",
		},
	}

	writeJSON(w, http.StatusOK, settings)
}
