package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tpt-online-video/packages/storage"
)

// UploadExpirationCleanup periodically marks expired upload sessions and cleans up storage.
type UploadExpirationCleanup struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	storage storage.Provider
}

func NewUploadExpirationCleanup(logger *slog.Logger, db *pgxpool.Pool, store storage.Provider) *UploadExpirationCleanup {
	return &UploadExpirationCleanup{
		logger:  logger,
		db:      db,
		storage: store,
	}
}

// Run starts the background cleanup loop. It runs every interval and expires
// sessions that have exceeded their expires_at timestamp.
func (c *UploadExpirationCleanup) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.logger.Info("upload expiration cleanup started", "interval", interval.String())

	// Run once immediately
	c.expireSessions(ctx)

	for {
		select {
		case <-ticker.C:
			c.expireSessions(ctx)
		case <-ctx.Done():
			c.logger.Info("upload expiration cleanup stopped")
			return
		}
	}
}

func (c *UploadExpirationCleanup) expireSessions(ctx context.Context) {
	// Find expired sessions that are still in a non-terminal state
	rows, err := c.db.Query(ctx,
		`UPDATE upload_sessions
		 SET status = 'expired', updated_at = now()
		 WHERE expires_at < now()
		   AND status NOT IN ('complete', 'cancelled', 'expired', 'failed')
		 RETURNING id, raw_object_key`,
	)
	if err != nil {
		c.logger.Error("expire upload sessions query", "error", err)
		return
	}
	defer rows.Close()

	var expiredCount int
	for rows.Next() {
		var sessionID, rawObjectKey string
		if err := rows.Scan(&sessionID, &rawObjectKey); err != nil {
			c.logger.Error("scan expired upload session", "error", err)
			continue
		}
		expiredCount++

		// Clean up storage asynchronously
		if rawObjectKey != "" {
			go func(key string) {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := c.storage.DeleteObject(cleanupCtx, "tpt-media", key); err != nil {
					c.logger.Error("cleanup expired upload object", "error", err, "key", key)
				}
			}(rawObjectKey)
		}
		_ = sessionID
	}

	if expiredCount > 0 {
		c.logger.Info("expired upload sessions", "count", expiredCount)
	}
}