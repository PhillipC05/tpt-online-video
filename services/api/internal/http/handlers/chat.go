package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/services/api/internal/http/middleware"
	svclive "github.com/tpt-online-video/services/api/internal/live"
)

// ChatHandler handles live chat HTTP and WebSocket requests.
type ChatHandler struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	chat    *svclive.ChatService
	liveRepo *svclive.Repository // to verify stream ownership
}

// NewChatHandler creates a new chat handler.
func NewChatHandler(logger *slog.Logger, db *pgxpool.Pool, chat *svclive.ChatService, liveRepo *svclive.Repository) *ChatHandler {
	return &ChatHandler{
		logger:   logger,
		db:       db,
		chat:     chat,
		liveRepo: liveRepo,
	}
}

// ─── WebSocket ───────────────────────────────────────────────────────────────

// Connect upgrades the request to a WebSocket and joins the chat room.
// Authentication is optional: unauthenticated clients can read but not send.
func (h *ChatHandler) Connect(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")

	// Verify stream exists (no status restriction — allow watching history after end)
	stream, err := h.liveRepo.GetByID(r.Context(), streamID)
	if err != nil || stream == nil {
		writeError(w, http.StatusNotFound, "stream not found")
		return
	}

	userID := middleware.GetUserID(r)
	displayName := middleware.GetUserName(r)
	if displayName == "" {
		displayName = "Anonymous"
	}

	ws, err := svclive.UpgradeWebSocket(w, r)
	if err != nil {
		h.logger.Warn("websocket upgrade failed", "error", err)
		return // response already started or hijacked
	}

	// Send recent history on connect
	history, err := h.chat.ListMessages(r.Context(), streamID, "", 50)
	if err == nil && len(history) > 0 {
		payload, _ := svclive.EncodeChatEvent("history", map[string]any{"messages": history})
		ws.WriteText(payload)
	}

	// Send joined confirmation
	joinedPayload, _ := svclive.EncodeChatEvent("joined", map[string]string{
		"stream_id":    streamID,
		"user_id":      userID,
		"display_name": displayName,
	})
	ws.WriteText(joinedPayload)

	client := svclive.NewChatClient(ws, streamID, userID, displayName)

	// Run client — blocks until disconnect (use background context so server shutdown
	// doesn't abruptly cut the connection without the close frame)
	h.chat.HandleClient(context.Background(), client)
}

// ─── History ─────────────────────────────────────────────────────────────────

// ListMessages returns paginated chat message history for a stream.
func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")

	before := r.URL.Query().Get("before")
	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if n, err := strconv.Atoi(lStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	msgs, err := h.chat.ListMessages(r.Context(), streamID, before, limit)
	if err != nil {
		h.logger.Error("list chat messages", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to load messages")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": msgs})
}

// ─── Moderation (stream owner or mod/admin) ───────────────────────────────────

// DeleteMessage soft-deletes a chat message.
func (h *ChatHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	messageID := chi.URLParam(r, "messageID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	if err := h.chat.DeleteMessage(r.Context(), messageID, callerID); err != nil {
		h.logger.Error("delete chat message", "error", err, "stream_id", streamID)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message_id": messageID})
}

// TimeoutUser mutes a user in the stream's chat.
func (h *ChatHandler) TimeoutUser(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	targetUserID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	var req struct {
		DurationSeconds int    `json:"duration_seconds"`
		Reason          string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 300
	}

	if err := h.chat.TimeoutUser(r.Context(), streamID, targetUserID, callerID, req.DurationSeconds); err != nil {
		h.logger.Error("timeout chat user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to timeout user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user_id":          targetUserID,
		"duration_seconds": req.DurationSeconds,
	})
}

// RemoveTimeout removes a user's chat timeout.
func (h *ChatHandler) RemoveTimeout(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	targetUserID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	if err := h.chat.RemoveTimeout(r.Context(), streamID, targetUserID); err != nil {
		h.logger.Error("remove chat timeout", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to remove timeout")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": targetUserID})
}

// BanUser permanently bans a user from the stream's chat.
func (h *ChatHandler) BanUser(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	targetUserID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if err := h.chat.BanUser(r.Context(), streamID, targetUserID, callerID, req.Reason); err != nil {
		h.logger.Error("ban chat user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to ban user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": targetUserID})
}

// UnbanUser removes a chat ban.
func (h *ChatHandler) UnbanUser(w http.ResponseWriter, r *http.Request) {
	streamID := chi.URLParam(r, "streamID")
	targetUserID := chi.URLParam(r, "userID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	if err := h.chat.UnbanUser(r.Context(), streamID, targetUserID); err != nil {
		h.logger.Error("unban chat user", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to unban user")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"user_id": targetUserID})
}

// LockChat locks or unlocks chat for the stream.
func (h *ChatHandler) LockChat(w http.ResponseWriter, r *http.Request) {
	h.setChatLock(w, r, true)
}

// UnlockChat re-opens chat for the stream.
func (h *ChatHandler) UnlockChat(w http.ResponseWriter, r *http.Request) {
	h.setChatLock(w, r, false)
}

func (h *ChatHandler) setChatLock(w http.ResponseWriter, r *http.Request, locked bool) {
	streamID := chi.URLParam(r, "streamID")
	callerID := middleware.GetUserID(r)

	if !h.callerCanModerate(r, streamID, callerID) {
		writeError(w, http.StatusForbidden, "not authorized to moderate this chat")
		return
	}

	if err := h.chat.SetChatLocked(r.Context(), streamID, locked); err != nil {
		h.logger.Error("set chat lock", "error", err, "locked", locked)
		writeError(w, http.StatusInternalServerError, "failed to update chat lock")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"locked": locked})
}

// ─── Reports ─────────────────────────────────────────────────────────────────

// ReportMessage is handled by the ModerationHandler via /api/v1/live/chat/{messageID}/report.
// No action needed here — the report target_type "live_chat_message" is already registered.

// ─── Internal helpers ─────────────────────────────────────────────────────────

// callerCanModerate returns true if callerID is the stream owner OR has mod/admin role.
func (h *ChatHandler) callerCanModerate(r *http.Request, streamID, callerID string) bool {
	if callerID == "" {
		return false
	}
	// Admins/mods are always allowed (role comes from JWT)
	role := middleware.GetUserRole(r)
	if role == "admin" || role == "moderator" {
		return true
	}
	// Otherwise must be stream owner
	stream, err := h.liveRepo.GetByID(r.Context(), streamID)
	if err != nil || stream == nil {
		return false
	}
	return stream.OwnerID == callerID
}
