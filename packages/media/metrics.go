package media

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// QueueMetrics is a snapshot of queue state at a point in time.
type QueueMetrics struct {
	// StreamLength is the total number of messages in the stream (includes all
	// entries, both delivered and not yet delivered).
	StreamLength int64

	// PendingCount is the number of messages claimed by a consumer but not yet
	// acknowledged (i.e., currently being processed or awaiting retry).
	PendingCount int64

	// UndeliveredCount is the consumer-group lag: messages in the stream not
	// yet delivered to any consumer. This is the primary "backlog" metric.
	UndeliveredCount int64

	// ConsumerCount is the number of registered consumers in the group.
	ConsumerCount int64

	// DLQLength is the number of messages in the dead-letter stream.
	DLQLength int64

	// OldestPendingAge is the wall-clock age of the oldest unacknowledged
	// (pending) message, derived from its Redis stream ID timestamp.
	OldestPendingAge time.Duration
}

// Metrics queries Redis for a current snapshot of queue state.
func (q *Queue) Metrics(ctx context.Context) (*QueueMetrics, error) {
	pipe := q.client.Pipeline()
	streamLenCmd := pipe.XLen(ctx, q.streamKey)
	dlqLenCmd := pipe.XLen(ctx, q.dlqKey)
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, fmt.Errorf("xlen pipeline: %w", err)
	}

	// XINFO GROUPS gives us lag (undelivered) and pending counts per group.
	var (
		pendingCount     int64
		undeliveredCount int64
		consumerCount    int64
	)
	groups, err := q.client.XInfoGroups(ctx, q.streamKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("xinfo groups: %w", err)
	}
	for _, g := range groups {
		if g.Name == q.groupName {
			pendingCount = g.Pending
			consumerCount = g.Consumers
			if g.Lag >= 0 {
				undeliveredCount = g.Lag
			}
			break
		}
	}

	// XPENDING summary for the age of the oldest pending message.
	var oldestAge time.Duration
	pending, err := q.client.XPending(ctx, q.streamKey, q.groupName).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("xpending: %w", err)
	}
	if pending != nil && pending.Count > 0 && pending.Lower != "" {
		var tsMS int64
		fmt.Sscanf(pending.Lower, "%d-", &tsMS)
		if tsMS > 0 {
			oldestAge = time.Since(time.UnixMilli(tsMS))
		}
	}

	return &QueueMetrics{
		StreamLength:     streamLenCmd.Val(),
		PendingCount:     pendingCount,
		UndeliveredCount: undeliveredCount,
		ConsumerCount:    consumerCount,
		DLQLength:        dlqLenCmd.Val(),
		OldestPendingAge: oldestAge,
	}, nil
}
