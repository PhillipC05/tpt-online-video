package live

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChatMessage is a persisted live chat message.
type ChatMessage struct {
	ID          string     `json:"id"`
	StreamID    string     `json:"stream_id"`
	UserID      *string    `json:"user_id,omitempty"`
	DisplayName string     `json:"display_name"`
	Body        string     `json:"body"`
	Deleted     bool       `json:"deleted"`
	DeletedBy   *string    `json:"deleted_by,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// ChatRepository handles database operations for live chat.
type ChatRepository struct {
	db *pgxpool.Pool
}

// NewChatRepository creates a new chat repository.
func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

// CreateMessage inserts a new chat message and returns it.
func (r *ChatRepository) CreateMessage(ctx context.Context, streamID string, userID *string, displayName, body string) (*ChatMessage, error) {
	m := &ChatMessage{
		StreamID:    streamID,
		UserID:      userID,
		DisplayName: displayName,
		Body:        body,
	}
	err := r.db.QueryRow(ctx,
		`INSERT INTO live_chat_messages (stream_id, user_id, display_name, body)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		streamID, userID, displayName, body,
	).Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ListMessages returns up to limit messages for a stream, ordered newest-first.
// If before is non-empty it is used as a cursor (messages created before that message ID).
func (r *ChatRepository) ListMessages(ctx context.Context, streamID, before string, limit int) ([]*ChatMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if before == "" {
		rows, err = r.db.Query(ctx,
			`SELECT id, stream_id, user_id, display_name, body, deleted, deleted_by, deleted_at, created_at
			 FROM live_chat_messages
			 WHERE stream_id = $1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			streamID, limit,
		)
	} else {
		rows, err = r.db.Query(ctx,
			`SELECT id, stream_id, user_id, display_name, body, deleted, deleted_by, deleted_at, created_at
			 FROM live_chat_messages
			 WHERE stream_id = $1 AND created_at < (
			   SELECT created_at FROM live_chat_messages WHERE id = $2
			 )
			 ORDER BY created_at DESC
			 LIMIT $3`,
			streamID, before, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*ChatMessage
	for rows.Next() {
		m := &ChatMessage{}
		if err := rows.Scan(&m.ID, &m.StreamID, &m.UserID, &m.DisplayName, &m.Body,
			&m.Deleted, &m.DeletedBy, &m.DeletedAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		// Redact body of deleted messages
		if m.Deleted {
			m.Body = ""
		}
		msgs = append(msgs, m)
	}
	if msgs == nil {
		msgs = []*ChatMessage{}
	}
	return msgs, nil
}

// DeleteMessage soft-deletes a chat message.
func (r *ChatRepository) DeleteMessage(ctx context.Context, messageID, deletedBy string) (bool, error) {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx,
		`UPDATE live_chat_messages
		 SET deleted = TRUE, deleted_by = $1, deleted_at = $2
		 WHERE id = $3 AND deleted = FALSE`,
		deletedBy, now, messageID,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// GetMessageStreamID returns the stream_id for a given message ID.
func (r *ChatRepository) GetMessageStreamID(ctx context.Context, messageID string) (string, error) {
	var streamID string
	err := r.db.QueryRow(ctx,
		`SELECT stream_id FROM live_chat_messages WHERE id = $1`,
		messageID,
	).Scan(&streamID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return streamID, nil
}

// BanUser inserts or replaces a chat ban for the given user in the given stream.
func (r *ChatRepository) BanUser(ctx context.Context, streamID, userID, bannedBy, reason string) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO live_chat_bans (stream_id, user_id, banned_by, reason)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (stream_id, user_id) DO UPDATE
		   SET banned_by = EXCLUDED.banned_by,
		       reason = EXCLUDED.reason,
		       created_at = NOW()`,
		streamID, userID, bannedBy, reason,
	)
	return err
}

// UnbanUser removes a chat ban.
func (r *ChatRepository) UnbanUser(ctx context.Context, streamID, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM live_chat_bans WHERE stream_id = $1 AND user_id = $2`,
		streamID, userID,
	)
	return err
}

// IsBanned reports whether the user is banned from the stream's chat.
func (r *ChatRepository) IsBanned(ctx context.Context, streamID, userID string) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM live_chat_bans WHERE stream_id = $1 AND user_id = $2)`,
		streamID, userID,
	).Scan(&exists)
	return exists, err
}

// TimeoutUser upserts a timeout for the user in the stream's chat.
func (r *ChatRepository) TimeoutUser(ctx context.Context, streamID, userID, timedOutBy string, durationSecs int) (*time.Time, error) {
	expiresAt := time.Now().UTC().Add(time.Duration(durationSecs) * time.Second)
	_, err := r.db.Exec(ctx,
		`INSERT INTO live_chat_timeouts (stream_id, user_id, timed_out_by, duration_seconds, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (stream_id, user_id) DO UPDATE
		   SET timed_out_by = EXCLUDED.timed_out_by,
		       duration_seconds = EXCLUDED.duration_seconds,
		       expires_at = EXCLUDED.expires_at,
		       created_at = NOW()`,
		streamID, userID, timedOutBy, durationSecs, expiresAt,
	)
	if err != nil {
		return nil, err
	}
	return &expiresAt, nil
}

// RemoveTimeout removes any active timeout for the user in the stream's chat.
func (r *ChatRepository) RemoveTimeout(ctx context.Context, streamID, userID string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM live_chat_timeouts WHERE stream_id = $1 AND user_id = $2`,
		streamID, userID,
	)
	return err
}

// IsTimedOut reports whether the user currently has an active timeout in the stream's chat.
func (r *ChatRepository) IsTimedOut(ctx context.Context, streamID, userID string) (bool, *time.Time, error) {
	var expiresAt time.Time
	err := r.db.QueryRow(ctx,
		`SELECT expires_at FROM live_chat_timeouts
		 WHERE stream_id = $1 AND user_id = $2 AND expires_at > NOW()`,
		streamID, userID,
	).Scan(&expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil, nil
		}
		return false, nil, err
	}
	return true, &expiresAt, nil
}

// SetChatLocked sets or clears the chat_locked flag on the stream.
func (r *ChatRepository) SetChatLocked(ctx context.Context, streamID string, locked bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE live_streams SET chat_locked = $1, updated_at = NOW() WHERE id = $2`,
		locked, streamID,
	)
	return err
}

// IsChatLocked returns whether the stream's chat is locked.
func (r *ChatRepository) IsChatLocked(ctx context.Context, streamID string) (bool, error) {
	var locked bool
	err := r.db.QueryRow(ctx,
		`SELECT chat_locked FROM live_streams WHERE id = $1`,
		streamID,
	).Scan(&locked)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return locked, nil
}
