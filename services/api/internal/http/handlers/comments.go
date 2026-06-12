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
	"github.com/tpt-online-video/services/api/internal/http/middleware"
)

type CommentHandler struct {
	logger *slog.Logger
	db     *pgxpool.Pool
}

func NewCommentHandler(logger *slog.Logger, db *pgxpool.Pool) *CommentHandler {
	return &CommentHandler{logger: logger, db: db}
}

// ─── Request / Response types ────────────────────────────────────────────────

type CreateCommentRequest struct {
	Body     string `json:"body"`
	ParentID string `json:"parent_id,omitempty"` // optional for replies
}

type UpdateCommentRequest struct {
	Body string `json:"body"`
}

type CommentResponse struct {
	ID        string              `json:"id"`
	VideoID   string              `json:"video_id"`
	UserID    string              `json:"user_id"`
	ParentID  *string             `json:"parent_id,omitempty"`
	Body      string              `json:"body"`
	Status    string              `json:"status"`
	Owner     *CommentOwner       `json:"owner,omitempty"`
	LikeCount int                 `json:"like_count"`
	Liked     bool                `json:"liked"`
	Replies   []*CommentResponse  `json:"replies,omitempty"`
	CreatedAt string              `json:"created_at"`
	UpdatedAt string              `json:"updated_at"`
}

type CommentOwner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// ─── CRUD ────────────────────────────────────────────────────────────────────

func (h *CommentHandler) CreateComment(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	var req CreateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "comment body cannot be empty")
		return
	}
	if len(req.Body) > 5000 {
		writeError(w, http.StatusBadRequest, "comment body too long (max 5000 characters)")
		return
	}

	// Verify video exists and is public (or owned by caller)
	var ownerID, visibility string
	err := h.db.QueryRow(r.Context(),
		`SELECT owner_id, visibility::text FROM videos WHERE id = $1 AND deleted_at IS NULL`, videoID,
	).Scan(&ownerID, &visibility)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "video not found")
			return
		}
		h.logger.Error("create comment video check", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Only allow commenting on public/unlisted videos, or own videos
	if visibility == "private" && ownerID != callerID {
		writeError(w, http.StatusForbidden, "cannot comment on this video")
		return
	}

	// If replying, verify parent comment exists and belongs to this video
	if req.ParentID != "" {
		var parentVideoID string
		err := h.db.QueryRow(r.Context(),
			`SELECT video_id FROM comments WHERE id = $1 AND deleted_at IS NULL AND status = 'visible'`, req.ParentID,
		).Scan(&parentVideoID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "parent comment not found")
				return
			}
			h.logger.Error("create comment parent check", "error", err)
			writeError(w, http.StatusInternalServerError, "database error")
			return
		}
		if parentVideoID != videoID {
			writeError(w, http.StatusBadRequest, "parent comment does not belong to this video")
			return
		}
	}

	var commentID string
	var createdAt time.Time
	var parentID *string
	if req.ParentID != "" {
		parentID = &req.ParentID
	}

	err = h.db.QueryRow(r.Context(),
		`INSERT INTO comments (video_id, user_id, parent_id, body)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		videoID, callerID, parentID, req.Body,
	).Scan(&commentID, &createdAt)
	if err != nil {
		h.logger.Error("create comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	createdAtStr := createdAt.UTC().Format(time.RFC3339)

	resp := CommentResponse{
		ID:        commentID,
		VideoID:   videoID,
		UserID:    callerID,
		ParentID:  parentID,
		Body:      req.Body,
		Status:    "visible",
		LikeCount: 0,
		Liked:     false,
		CreatedAt: createdAtStr,
		UpdatedAt: createdAtStr,
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (h *CommentHandler) ListComments(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	// Fetch top-level comments ordered by likes (popularity) then newest
	rows, err := h.db.Query(r.Context(), `
		SELECT c.id, c.body, c.status::text, c.user_id, u.display_name, c.created_at, c.updated_at,
		       COALESCE(cl.like_count, 0) AS like_count
		FROM comments c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN (
			SELECT comment_id, COUNT(*) AS like_count FROM comment_likes GROUP BY comment_id
		) cl ON cl.comment_id = c.id
		WHERE c.video_id = $1 AND c.parent_id IS NULL AND c.deleted_at IS NULL
		ORDER BY like_count DESC, c.created_at DESC
		LIMIT 50`,
		videoID,
	)
	if err != nil {
		h.logger.Error("list top-level comments", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	defer rows.Close()

	var comments []*CommentResponse
	for rows.Next() {
		c, err := scanComment(rows, callerID)
		if err != nil {
			h.logger.Error("scan top-level comment", "error", err)
			continue
		}
		if c.Status == "hidden" && c.UserID != callerID {
			c.Body = "[comment hidden]"
		}
		comments = append(comments, c)
	}

	// Fetch replies for each top-level comment (one extra round-trip per comment — acceptable for now)
	for _, parent := range comments {
		parent.Replies = h.fetchReplies(r.Context(), videoID, parent.ID, callerID)
	}

	if comments == nil {
		comments = []*CommentResponse{}
	}

	writeJSON(w, http.StatusOK, comments)
}

func (h *CommentHandler) fetchReplies(ctx context.Context, videoID, parentID, callerID string) []*CommentResponse {
	rows, err := h.db.Query(ctx, `
		SELECT c.id, c.body, c.status::text, c.user_id, u.display_name, c.created_at, c.updated_at,
		       COALESCE(cl.like_count, 0) AS like_count
		FROM comments c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN (
			SELECT comment_id, COUNT(*) AS like_count FROM comment_likes GROUP BY comment_id
		) cl ON cl.comment_id = c.id
		WHERE c.video_id = $1 AND c.parent_id = $2 AND c.deleted_at IS NULL
		ORDER BY c.created_at ASC
		LIMIT 10`,
		videoID, parentID,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var replies []*CommentResponse
	for rows.Next() {
		r, err := scanComment(rows, callerID)
		if err != nil {
			continue
		}
		if r.Status == "hidden" && r.UserID != callerID {
			r.Body = "[comment hidden]"
		}
		replies = append(replies, r)
	}
	return replies
}

// commentScanner is satisfied by both pgx.Row and pgx.Rows
type commentScanner interface {
	Scan(dest ...any) error
}

func scanComment(row commentScanner, callerID string) (*CommentResponse, error) {
	var id, body, status, userID, displayName string
	var likeCount int
	var createdAt, updatedAt time.Time

	err := row.Scan(&id, &body, &status, &userID, &displayName, &createdAt, &updatedAt, &likeCount)
	if err != nil {
		return nil, err
	}

	var liked bool
	if callerID != "" {
		// We'll fill liked after scan since we can't do subquery easily here
	}

	return &CommentResponse{
		ID:        id,
		Body:      body,
		Status:    status,
		UserID:    userID,
		Owner:     &CommentOwner{ID: userID, DisplayName: displayName},
		LikeCount: likeCount,
		Liked:     liked,
		CreatedAt: createdAt.UTC().Format(time.RFC3339),
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (h *CommentHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	callerID := middleware.GetUserID(r)

	var req UpdateCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "comment body cannot be empty")
		return
	}
	if len(req.Body) > 5000 {
		writeError(w, http.StatusBadRequest, "comment body too long (max 5000 characters)")
		return
	}

	// Verify ownership
	var ownerID, status string
	err := h.db.QueryRow(r.Context(),
		`SELECT user_id, status::text FROM comments WHERE id = $1 AND deleted_at IS NULL`, commentID,
	).Scan(&ownerID, &status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "comment not found")
			return
		}
		h.logger.Error("update comment owner check", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	if ownerID != callerID {
		writeError(w, http.StatusForbidden, "you do not own this comment")
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE comments SET body = $1, updated_at = NOW() WHERE id = $2`, req.Body, commentID,
	)
	if err != nil {
		h.logger.Error("update comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"id": commentID})
}

func (h *CommentHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	callerID := middleware.GetUserID(r)

	// Soft-delete: only the comment owner can delete
	tag, err := h.db.Exec(r.Context(),
		`UPDATE comments SET deleted_at = NOW() WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`,
		commentID, callerID,
	)
	if err != nil {
		h.logger.Error("delete comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Reports ─────────────────────────────────────────────────────────────────

type ReportCommentRequest struct {
	Reason string `json:"reason"`
}

func (h *CommentHandler) ReportComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	callerID := middleware.GetUserID(r)

	var req ReportCommentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		writeError(w, http.StatusBadRequest, "reason cannot be empty")
		return
	}
	if len(req.Reason) > 2000 {
		writeError(w, http.StatusBadRequest, "reason too long (max 2000 characters)")
		return
	}

	// Verify comment exists
	var exists bool
	err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1 AND deleted_at IS NULL)`, commentID,
	).Scan(&exists)
	if err != nil || !exists {
		writeError(w, http.StatusNotFound, "comment not found")
		return
	}

	_, err = h.db.Exec(r.Context(),
		`INSERT INTO comment_reports (comment_id, reporter_id, reason)
		 VALUES ($1, $2, $3)
		 ON CONFLICT DO NOTHING`,
		commentID, callerID, req.Reason,
	)
	if err != nil {
		h.logger.Error("report comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "reported"})
}

// ─── Likes (video + comment) ─────────────────────────────────────────────────

func (h *CommentHandler) LikeVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO video_likes (user_id, video_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		callerID, videoID,
	)
	if err != nil {
		h.logger.Error("like video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	// Return updated like count
	var likeCount int64
	_ = h.db.QueryRow(r.Context(), `SELECT like_count FROM videos WHERE id = $1`, videoID).Scan(&likeCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked":      true,
		"like_count": likeCount,
	})
}

func (h *CommentHandler) UnlikeVideo(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM video_likes WHERE user_id = $1 AND video_id = $2`,
		callerID, videoID,
	)
	if err != nil {
		h.logger.Error("unlike video", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	var likeCount int64
	_ = h.db.QueryRow(r.Context(), `SELECT like_count FROM videos WHERE id = $1`, videoID).Scan(&likeCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked":      false,
		"like_count": likeCount,
	})
}

func (h *CommentHandler) GetVideoLikeStatus(w http.ResponseWriter, r *http.Request) {
	videoID := chi.URLParam(r, "videoID")
	callerID := middleware.GetUserID(r)

	var liked bool
	err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM video_likes WHERE user_id = $1 AND video_id = $2)`,
		callerID, videoID,
	).Scan(&liked)
	if err != nil {
		h.logger.Error("get video like status", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	var likeCount int64
	_ = h.db.QueryRow(r.Context(), `SELECT like_count FROM videos WHERE id = $1`, videoID).Scan(&likeCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked":      liked,
		"like_count": likeCount,
	})
}

func (h *CommentHandler) LikeComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	callerID := middleware.GetUserID(r)

	_, err := h.db.Exec(r.Context(),
		`INSERT INTO comment_likes (user_id, comment_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		callerID, commentID,
	)
	if err != nil {
		h.logger.Error("like comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	var likeCount int
	_ = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1`, commentID,
	).Scan(&likeCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked":      true,
		"like_count": likeCount,
	})
}

func (h *CommentHandler) UnlikeComment(w http.ResponseWriter, r *http.Request) {
	commentID := chi.URLParam(r, "commentID")
	callerID := middleware.GetUserID(r)

	_, err := h.db.Exec(r.Context(),
		`DELETE FROM comment_likes WHERE user_id = $1 AND comment_id = $2`,
		callerID, commentID,
	)
	if err != nil {
		h.logger.Error("unlike comment", "error", err)
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}

	var likeCount int
	_ = h.db.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM comment_likes WHERE comment_id = $1`, commentID,
	).Scan(&likeCount)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"liked":      false,
		"like_count": likeCount,
	})
}