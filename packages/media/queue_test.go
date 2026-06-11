package media

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisTestClient returns a Redis client for integration tests.
// The test is skipped if REDIS_TEST_ADDR is not set or Redis is unreachable.
func redisTestClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("REDIS_TEST_ADDR")
	if addr == "" {
		t.Skip("REDIS_TEST_ADDR not set — skipping Redis integration tests")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable at %s: %v", addr, err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// newTestQueue creates a Queue backed by a unique stream key for the test.
// It also flushes the stream on cleanup.
func newTestQueue(t *testing.T, client *redis.Client) *Queue {
	t.Helper()
	stream := "test:queue:" + t.Name()
	group := "test-group"
	consumer := "test-worker"
	q := NewQueue(client, stream, group, consumer)
	ctx := context.Background()
	if err := q.EnsureGroup(ctx); err != nil {
		t.Fatalf("EnsureGroup: %v", err)
	}
	t.Cleanup(func() {
		client.Del(ctx, stream, q.DLQKey())
	})
	return q
}

func testJob(id string) *Job {
	return &Job{
		ID:          id,
		VideoID:     "vid-" + id,
		RawObjectKey: "raw/" + id,
		OwnerID:     "owner-1",
		CreatedAt:   time.Now().Unix(),
		Attempt:     0,
		MaxAttempts: 3,
	}
}

func TestEnqueueAndClaim(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	job := testJob("job-1")
	msgID, err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if msgID == "" {
		t.Fatal("expected non-empty message ID")
	}

	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil {
		t.Fatalf("ClaimPending: %v", err)
	}
	if result == nil {
		t.Fatal("expected a claimed job, got nil")
	}
	if result.Job.ID != job.ID {
		t.Errorf("expected job ID %q, got %q", job.ID, result.Job.ID)
	}
}

func TestClaimPending_NoJobs(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	result, err := q.ClaimPending(ctx, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("ClaimPending: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result on empty queue, got %+v", result)
	}
}

func TestAck(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	_, err := q.Enqueue(ctx, testJob("job-ack"))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result == nil {
		t.Fatalf("ClaimPending: err=%v result=%v", err, result)
	}

	if err := q.Ack(ctx, result.MessageID); err != nil {
		t.Fatalf("Ack: %v", err)
	}

	// Pending count should be zero after ack.
	pending, err := client.XPending(ctx, q.streamKey, q.groupName).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after ack, got %d", pending.Count)
	}
}

func TestNackWithAttempt_Retry(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	job := testJob("job-retry")
	job.MaxAttempts = 3

	_, err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result == nil {
		t.Fatalf("ClaimPending: err=%v result=%v", err, result)
	}

	// First nack — should re-enqueue (attempt 1, MaxAttempts=3).
	if err := q.NackWithAttempt(ctx, result.MessageID, result.Job, "transient error"); err != nil {
		t.Fatalf("NackWithAttempt: %v", err)
	}

	// Claim the re-enqueued message.
	result2, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result2 == nil {
		t.Fatalf("ClaimPending after nack: err=%v result=%v", err, result2)
	}
	if result2.Job.Attempt != 1 {
		t.Errorf("expected attempt=1 after first nack, got %d", result2.Job.Attempt)
	}
}

func TestNackWithAttempt_DeadLetter(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	job := testJob("job-dlq")
	job.MaxAttempts = 1 // one attempt → next nack goes straight to DLQ
	job.Attempt = 0

	_, err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result == nil {
		t.Fatalf("ClaimPending: err=%v result=%v", err, result)
	}

	if err := q.NackWithAttempt(ctx, result.MessageID, result.Job, "fatal error"); err != nil {
		t.Fatalf("NackWithAttempt to DLQ: %v", err)
	}

	// DLQ stream should have one entry.
	dlqMsgs, err := client.XRange(ctx, q.DLQKey(), "-", "+").Result()
	if err != nil {
		t.Fatalf("XRange DLQ: %v", err)
	}
	if len(dlqMsgs) != 1 {
		t.Fatalf("expected 1 DLQ entry, got %d", len(dlqMsgs))
	}

	var entry DeadLetterEntry
	payload := dlqMsgs[0].Values["payload"].(string)
	if err := json.Unmarshal([]byte(payload), &entry); err != nil {
		t.Fatalf("unmarshal DLQ entry: %v", err)
	}
	if entry.Reason != "fatal error" {
		t.Errorf("expected reason %q, got %q", "fatal error", entry.Reason)
	}
	if entry.Job.ID != job.ID {
		t.Errorf("expected job ID %q in DLQ, got %q", job.ID, entry.Job.ID)
	}
}

func TestMoveToDeadLetter(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	job := testJob("job-manual-dlq")
	_, err := q.Enqueue(ctx, job)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result == nil {
		t.Fatalf("ClaimPending: err=%v result=%v", err, result)
	}

	if err := q.MoveToDeadLetter(ctx, result.MessageID, result.Job, "manual dead-letter"); err != nil {
		t.Fatalf("MoveToDeadLetter: %v", err)
	}

	// Original message should be acked (PEL empty).
	pending, err := client.XPending(ctx, q.streamKey, q.groupName).Result()
	if err != nil {
		t.Fatalf("XPending: %v", err)
	}
	if pending.Count != 0 {
		t.Errorf("expected 0 pending after MoveToDeadLetter, got %d", pending.Count)
	}

	// DLQ should have the entry.
	dlqLen, err := client.XLen(ctx, q.DLQKey()).Result()
	if err != nil {
		t.Fatalf("XLen DLQ: %v", err)
	}
	if dlqLen != 1 {
		t.Errorf("expected 1 DLQ entry, got %d", dlqLen)
	}
}

func TestHeartbeat(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	if err := q.Heartbeat(ctx, 2); err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	key := "worker:heartbeat:" + q.consumer
	t.Cleanup(func() { client.Del(ctx, key) })

	val, err := client.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("Get heartbeat key: %v", err)
	}

	var hb map[string]any
	if err := json.Unmarshal([]byte(val), &hb); err != nil {
		t.Fatalf("unmarshal heartbeat: %v", err)
	}
	if hb["consumer"] != q.consumer {
		t.Errorf("expected consumer %q, got %v", q.consumer, hb["consumer"])
	}

	// TTL should be set (key will expire).
	ttl, err := client.TTL(ctx, key).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 {
		t.Error("expected positive TTL on heartbeat key")
	}
}

func TestQueueMetrics(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)
	ctx := context.Background()

	// Enqueue two jobs.
	for i := 0; i < 2; i++ {
		if _, err := q.Enqueue(ctx, testJob("metrics-job")); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	m, err := q.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}

	if m.StreamLength < 2 {
		t.Errorf("expected StreamLength >= 2, got %d", m.StreamLength)
	}
	if m.UndeliveredCount < 2 {
		t.Errorf("expected UndeliveredCount >= 2, got %d", m.UndeliveredCount)
	}

	// Claim one to move it from undelivered to pending.
	result, err := q.ClaimPending(ctx, 2*time.Second)
	if err != nil || result == nil {
		t.Fatalf("ClaimPending: err=%v result=%v", err, result)
	}

	m2, err := q.Metrics(ctx)
	if err != nil {
		t.Fatalf("Metrics after claim: %v", err)
	}
	if m2.PendingCount != 1 {
		t.Errorf("expected PendingCount=1 after one claim, got %d", m2.PendingCount)
	}
	if m2.ConsumerCount < 1 {
		t.Errorf("expected ConsumerCount >= 1, got %d", m2.ConsumerCount)
	}
}

func TestScalingController_ScaleUp(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)

	cfg := ScalerConfig{
		MinWorkers:          1,
		MaxWorkers:          4,
		ScaleUpQueueDepth:   2,
		ScaleDownQueueDepth: 0,
		ScaleUpBusyPct:      0.9,
		ScaleDownBusyPct:    0.1,
		EvalInterval:        time.Minute, // we'll trigger manually
		CooldownUp:          0,
		CooldownDown:        time.Minute,
	}

	sc := NewScalingController(q, cfg, nil)
	ctx := context.Background()

	// Enqueue 3 jobs to exceed ScaleUpQueueDepth.
	for i := 0; i < 3; i++ {
		if _, err := q.Enqueue(ctx, testJob("scale-job")); err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
	}

	before := sc.Desired()
	sc.evaluate(ctx)
	after := sc.Desired()

	if after <= before {
		t.Errorf("expected scale-up: before=%d after=%d", before, after)
	}
}

func TestScalingController_ScaleDown(t *testing.T) {
	client := redisTestClient(t)
	q := newTestQueue(t, client)

	cfg := ScalerConfig{
		MinWorkers:          1,
		MaxWorkers:          4,
		ScaleUpQueueDepth:   10,
		ScaleDownQueueDepth: 1,
		ScaleUpBusyPct:      0.9,
		ScaleDownBusyPct:    0.5,
		EvalInterval:        time.Minute,
		CooldownUp:          time.Minute,
		CooldownDown:        0,
	}

	sc := NewScalingController(q, cfg, nil)
	sc.desired.Store(3) // start at 3 workers
	ctx := context.Background()

	// Empty queue, low utilization — should scale down.
	sc.evaluate(ctx)

	if sc.Desired() >= 3 {
		t.Errorf("expected scale-down from 3, got %d", sc.Desired())
	}
}
