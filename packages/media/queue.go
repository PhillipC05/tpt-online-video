package media

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const heartbeatTTL = 60 * time.Second

// Job represents a transcoding job in the queue.
type Job struct {
	ID              string `json:"id"`
	VideoID         string `json:"video_id"`
	UploadSessionID string `json:"upload_session_id"`
	RawObjectKey    string `json:"raw_object_key"`
	OwnerID         string `json:"owner_id"`
	CreatedAt       int64  `json:"created_at"`
	Attempt         int    `json:"attempt"`
	MaxAttempts     int    `json:"max_attempts"`
}

// Queue provides an abstraction over Redis Streams for transcoding jobs.
type Queue struct {
	client    *redis.Client
	streamKey string
	groupName string
	consumer  string
	dlqKey    string
}

// NewQueue creates a new Queue backed by Redis Streams.
// The dead-letter stream key is derived as streamKey + ":dlq".
func NewQueue(client *redis.Client, streamKey, groupName, consumer string) *Queue {
	return &Queue{
		client:    client,
		streamKey: streamKey,
		groupName: groupName,
		consumer:  consumer,
		dlqKey:    streamKey + ":dlq",
	}
}

// DeadLetterEntry wraps a Job that has exhausted all retry attempts.
type DeadLetterEntry struct {
	Job       *Job   `json:"job"`
	DeadAt    int64  `json:"dead_at"`
	Reason    string `json:"reason"`
	OrigMsgID string `json:"orig_msg_id"`
}

// DLQKey returns the dead-letter stream key.
func (q *Queue) DLQKey() string { return q.dlqKey }

// EnsureGroup creates the stream and consumer group if they don't exist.
func (q *Queue) EnsureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.streamKey, q.groupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("create consumer group: %w", err)
	}
	return nil
}

// Enqueue adds a job to the stream.
func (q *Queue) Enqueue(ctx context.Context, job *Job) (string, error) {
	data, err := json.Marshal(job)
	if err != nil {
		return "", fmt.Errorf("marshal job: %w", err)
	}
	msgID, err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.streamKey,
		Values: map[string]interface{}{"payload": string(data)},
	}).Result()
	if err != nil {
		return "", fmt.Errorf("xadd: %w", err)
	}
	return msgID, nil
}

// ClaimResult holds a claimed job and its Redis message ID.
type ClaimResult struct {
	MessageID string
	Job       *Job
}

// ClaimPending claims the next available job from the stream (blocking).
func (q *Queue) ClaimPending(ctx context.Context, timeout time.Duration) (*ClaimResult, error) {
	results, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.groupName,
		Consumer: q.consumer,
		Streams:  []string{q.streamKey, ">"},
		Count:    1,
		Block:    timeout,
	}).Result()
	if err != nil {
		return nil, err
	}
	if len(results) == 0 || len(results[0].Messages) == 0 {
		return nil, nil
	}

	msg := results[0].Messages[0]
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid payload in message %s", msg.ID)
	}

	var job Job
	if err := json.Unmarshal([]byte(payload), &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}

	return &ClaimResult{MessageID: msg.ID, Job: &job}, nil
}

// Ack acknowledges a job as completed, removing it from the pending list.
func (q *Queue) Ack(ctx context.Context, messageID string) error {
	return q.client.XAck(ctx, q.streamKey, q.groupName, messageID).Err()
}

// Nack moves a job back to the pending queue (for retry).
func (q *Queue) Nack(ctx context.Context, messageID string) error {
	// XPending with CLAIM with 0 min idle time re-delivers
	// Simpler: just XACK then re-enqueue with incremented attempt.
	// But the standard approach is to let PEL handle retries via XPENDING.
	// For now, we use XCLAIM with 0 idle time to re-deliver immediately.
	return q.client.XClaim(ctx, &redis.XClaimArgs{
		Stream:   q.streamKey,
		Group:    q.groupName,
		Consumer: q.consumer,
		MinIdle:  0,
		Messages: []string{messageID},
	}).Err()
}

// NackWithAttempt acks the original message, increments the attempt count on
// the job, and re-enqueues it for retry. If the job has reached MaxAttempts,
// it is moved to the dead-letter queue instead.
func (q *Queue) NackWithAttempt(ctx context.Context, messageID string, job *Job, reason string) error {
	job.Attempt++
	if job.MaxAttempts > 0 && job.Attempt >= job.MaxAttempts {
		return q.MoveToDeadLetter(ctx, messageID, job, reason)
	}
	// Ack original then re-enqueue with the updated attempt count.
	if err := q.Ack(ctx, messageID); err != nil {
		return fmt.Errorf("ack before retry: %w", err)
	}
	if _, err := q.Enqueue(ctx, job); err != nil {
		return fmt.Errorf("re-enqueue: %w", err)
	}
	return nil
}

// MoveToDeadLetter acks the message from the main stream and appends it to the
// dead-letter stream. Both operations execute in a single pipeline.
func (q *Queue) MoveToDeadLetter(ctx context.Context, messageID string, job *Job, reason string) error {
	entry := &DeadLetterEntry{
		Job:       job,
		DeadAt:    time.Now().Unix(),
		Reason:    reason,
		OrigMsgID: messageID,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal dlq entry: %w", err)
	}
	pipe := q.client.Pipeline()
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: q.dlqKey,
		Values: map[string]any{"payload": string(data)},
	})
	pipe.XAck(ctx, q.streamKey, q.groupName, messageID)
	_, err = pipe.Exec(ctx)
	return err
}

// Heartbeat writes a keep-alive record for this consumer to Redis with a TTL
// of 60 s. Expired keys indicate dead workers.
func (q *Queue) Heartbeat(ctx context.Context, activeJobs int) error {
	key := "worker:heartbeat:" + q.consumer
	data, _ := json.Marshal(map[string]any{
		"last_seen":   time.Now().Unix(),
		"consumer":    q.consumer,
		"stream":      q.streamKey,
		"active_jobs": activeJobs,
	})
	return q.client.Set(ctx, key, data, heartbeatTTL).Err()
}