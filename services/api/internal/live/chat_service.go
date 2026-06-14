package live

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// ChatService wires together the repository, hub, and Redis pub/sub for live chat.
type ChatService struct {
	repo   *ChatRepository
	hub    *ChatHub
	logger *slog.Logger
}

// NewChatService creates a new chat service.
func NewChatService(repo *ChatRepository, hub *ChatHub, logger *slog.Logger) *ChatService {
	return &ChatService{repo: repo, hub: hub, logger: logger}
}

// Hub returns the underlying ChatHub (used by the handler to call AddClient).
func (s *ChatService) Hub() *ChatHub { return s.hub }

// ─── Connection ──────────────────────────────────────────────────────────────

// HandleClient wires a connected WebSocket client into its room and handles I/O.
// It blocks until the client disconnects.
func (s *ChatService) HandleClient(ctx context.Context, c *ChatClient) {
	s.hub.AddClient(ctx, c, func(userID, body string) error {
		_, err := s.SendMessage(ctx, c.streamID, userID, c.displayName, body)
		return err
	})
}

// ─── Messaging ───────────────────────────────────────────────────────────────

// SendMessage validates, persists, and broadcasts a chat message.
func (s *ChatService) SendMessage(ctx context.Context, streamID, userID, displayName, body string) (*ChatMessage, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("message body cannot be empty")
	}
	if len(body) > 500 {
		return nil, fmt.Errorf("message too long (max 500 characters)")
	}

	// Check ban
	banned, err := s.repo.IsBanned(ctx, streamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check ban: %w", err)
	}
	if banned {
		return nil, fmt.Errorf("you are banned from this chat")
	}

	// Check timeout
	timedOut, expiresAt, err := s.repo.IsTimedOut(ctx, streamID, userID)
	if err != nil {
		return nil, fmt.Errorf("check timeout: %w", err)
	}
	if timedOut {
		return nil, fmt.Errorf("you are timed out until %s", expiresAt.Format(time.RFC3339))
	}

	// Check chat lock
	locked, err := s.repo.IsChatLocked(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("check chat lock: %w", err)
	}
	if locked {
		return nil, fmt.Errorf("chat is locked by the streamer")
	}

	uid := &userID
	msg, err := s.repo.CreateMessage(ctx, streamID, uid, displayName, body)
	if err != nil {
		return nil, fmt.Errorf("persist message: %w", err)
	}

	// Broadcast to all viewers via Redis
	payload, err := EncodeChatEvent("message", msg)
	if err != nil {
		return msg, nil
	}
	if pubErr := s.hub.Publish(ctx, streamID, payload); pubErr != nil {
		s.logger.Warn("chat publish failed", "stream_id", streamID, "error", pubErr)
	}
	s.hub.RecordMessage()
	return msg, nil
}

// ListMessages returns paginated chat history for a stream.
func (s *ChatService) ListMessages(ctx context.Context, streamID, before string, limit int) ([]*ChatMessage, error) {
	return s.repo.ListMessages(ctx, streamID, before, limit)
}

// ─── Moderation ──────────────────────────────────────────────────────────────

// DeleteMessage soft-deletes a chat message and notifies viewers.
// actorID must be the stream owner or a moderator/admin (enforced at the handler).
func (s *ChatService) DeleteMessage(ctx context.Context, messageID, actorID string) error {
	streamID, err := s.repo.GetMessageStreamID(ctx, messageID)
	if err != nil {
		return fmt.Errorf("get message stream: %w", err)
	}
	if streamID == "" {
		return fmt.Errorf("message not found")
	}

	deleted, err := s.repo.DeleteMessage(ctx, messageID, actorID)
	if err != nil {
		return fmt.Errorf("delete message: %w", err)
	}
	if !deleted {
		return fmt.Errorf("message not found or already deleted")
	}

	payload, _ := EncodeChatEvent("deleted", map[string]string{"message_id": messageID})
	if pubErr := s.hub.Publish(ctx, streamID, payload); pubErr != nil {
		s.logger.Warn("chat publish delete failed", "stream_id", streamID, "error", pubErr)
	}
	return nil
}

// TimeoutUser mutes a user in a stream's chat for durationSecs seconds.
func (s *ChatService) TimeoutUser(ctx context.Context, streamID, userID, actorID string, durationSecs int) error {
	if durationSecs <= 0 || durationSecs > 86400 {
		durationSecs = 300
	}

	expiresAt, err := s.repo.TimeoutUser(ctx, streamID, userID, actorID, durationSecs)
	if err != nil {
		return fmt.Errorf("timeout user: %w", err)
	}

	// Notify the affected user directly
	payload, _ := EncodeChatEvent("timed_out", map[string]string{
		"stream_id":  streamID,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
	s.hub.BroadcastToUser(streamID, userID, payload)
	return nil
}

// RemoveTimeout removes an active timeout for a user.
func (s *ChatService) RemoveTimeout(ctx context.Context, streamID, userID string) error {
	if err := s.repo.RemoveTimeout(ctx, streamID, userID); err != nil {
		return fmt.Errorf("remove timeout: %w", err)
	}
	payload, _ := EncodeChatEvent("timeout_removed", map[string]string{"stream_id": streamID})
	s.hub.BroadcastToUser(streamID, userID, payload)
	return nil
}

// BanUser permanently bans a user from a stream's chat.
func (s *ChatService) BanUser(ctx context.Context, streamID, userID, actorID, reason string) error {
	if err := s.repo.BanUser(ctx, streamID, userID, actorID, reason); err != nil {
		return fmt.Errorf("ban user: %w", err)
	}

	// Notify the banned user
	payload, _ := EncodeChatEvent("banned", map[string]string{"stream_id": streamID})
	s.hub.BroadcastToUser(streamID, userID, payload)
	return nil
}

// UnbanUser lifts a chat ban.
func (s *ChatService) UnbanUser(ctx context.Context, streamID, userID string) error {
	if err := s.repo.UnbanUser(ctx, streamID, userID); err != nil {
		return fmt.Errorf("unban user: %w", err)
	}
	return nil
}

// SetChatLocked locks or unlocks chat for a stream and notifies all viewers.
func (s *ChatService) SetChatLocked(ctx context.Context, streamID string, locked bool) error {
	if err := s.repo.SetChatLocked(ctx, streamID, locked); err != nil {
		return fmt.Errorf("set chat locked: %w", err)
	}

	eventType := "chat_locked"
	if !locked {
		eventType = "chat_unlocked"
	}
	payload, _ := EncodeChatEvent(eventType, map[string]bool{"locked": locked})
	if pubErr := s.hub.Publish(ctx, streamID, payload); pubErr != nil {
		s.logger.Warn("chat publish lock failed", "stream_id", streamID, "error", pubErr)
	}
	return nil
}
