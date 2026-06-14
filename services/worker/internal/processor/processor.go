package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/search"
	"github.com/tpt-online-video/packages/storage"
	"github.com/tpt-online-video/services/worker/internal/metrics"
)

type Processor struct {
	logger  *slog.Logger
	db      *pgxpool.Pool
	redis     *redis.Client
	storage   storage.Provider
	search    search.Provider
	queue     *media.Queue
	workDir string
	scaler  *media.ScalingController
	metrics *metrics.WorkerMetrics

	activeWorkers atomic.Int32
	totalDone     atomic.Int64
}

func New(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, searchProvider search.Provider, queue *media.Queue, workDir string) *Processor {
	return &Processor{
		logger:  logger,
		db:      db,
		redis:   redis,
		storage: store,
		search:  searchProvider,
		queue:   queue,
		workDir: workDir,
		metrics: metrics.New(),
	}
}

// Metrics returns the worker's operational metrics for external exposition.
func (p *Processor) Metrics() *metrics.WorkerMetrics { return p.metrics }

// WithScaler attaches a ScalingController whose Desired() value drives the
// runtime concurrency of the worker pool.
func (p *Processor) WithScaler(s *media.ScalingController) *Processor {
	p.scaler = s
	return p
}

// Run starts the worker pool, blocking until ctx is cancelled.
//
// initialConcurrency sets the starting concurrency. When a ScalingController is
// attached via WithScaler, the pool adjusts live between the scaler's Min and
// Max bounds; otherwise concurrency stays fixed at initialConcurrency.
func (p *Processor) Run(ctx context.Context, initialConcurrency int) {
	maxConc := initialConcurrency
	if p.scaler != nil && p.scaler.Config().MaxWorkers > maxConc {
		maxConc = p.scaler.Config().MaxWorkers
	}

	// sem is sized to the maximum possible concurrency so we never block on
	// sends; desired capacity is enforced by checking len(sem) < desired below.
	sem := make(chan struct{}, maxConc)

	go p.heartbeatLoop(ctx)
	if p.scaler != nil {
		go p.scaler.Run(ctx)
	}

	p.logger.Info("worker processor started", "concurrency", initialConcurrency, "max_concurrency", maxConc)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker processor stopping, draining active jobs")
			// Drain: acquire every slot to ensure all in-flight goroutines finish.
			for i := 0; i < maxConc; i++ {
				sem <- struct{}{}
			}
			return
		default:
		}

		desired := initialConcurrency
		if p.scaler != nil {
			desired = p.scaler.Desired()
			p.scaler.RecordUtilization(len(sem), desired)
		}

		if len(sem) >= desired {
			// At or above desired concurrency — wait for a slot to free up.
			time.Sleep(50 * time.Millisecond)
			continue
		}

		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			p.activeWorkers.Add(1)
			defer p.activeWorkers.Add(-1)
			if err := p.processNext(ctx); err != nil && !errors.Is(err, context.Canceled) {
				p.logger.Error("process next job", "error", err)
			}
		}()
	}
}

func (p *Processor) heartbeatLoop(ctx context.Context) {
	// Send an initial heartbeat immediately so the worker is visible at startup.
	_ = p.queue.Heartbeat(ctx, int(p.activeWorkers.Load()))

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = p.queue.Heartbeat(ctx, int(p.activeWorkers.Load()))
		}
	}
}

func (p *Processor) processNext(ctx context.Context) error {
	result, err := p.queue.ClaimPending(ctx, 5*time.Second)
	if err != nil {
		return fmt.Errorf("claim pending: %w", err)
	}
	if result == nil {
		return nil
	}

	job := result.Job
	p.logger.Info("processing job", "job_id", job.ID, "video_id", job.VideoID, "attempt", job.Attempt)

	p.metrics.RecordStart()
	start := time.Now()

	if err := p.processJob(ctx, result); err != nil {
		elapsed := time.Since(start).Milliseconds()
		isPerm := media.IsPermanent(err)
		p.metrics.RecordFailure(isPerm)
		p.logger.Error("job failed", "job_id", job.ID, "error", err, "attempt", job.Attempt, "elapsed_ms", elapsed)
		p.updateJobFailed(ctx, job.ID, err.Error())

		if isPerm {
			// Permanent failures skip retries and go straight to the dead-letter queue.
			p.logger.Warn("job permanently failed, moving to DLQ", "job_id", job.ID)
			if dlqErr := p.queue.MoveToDeadLetter(ctx, result.MessageID, job, err.Error()); dlqErr != nil {
				p.logger.Error("move to DLQ", "job_id", job.ID, "error", dlqErr)
			}
		} else {
			// Transient (or unclassified) failures: increment attempt and re-enqueue,
			// or DLQ if MaxAttempts is reached.
			if nackErr := p.queue.NackWithAttempt(ctx, result.MessageID, job, err.Error()); nackErr != nil {
				p.logger.Error("nack job", "job_id", job.ID, "error", nackErr)
			}
		}
		return err
	}

	elapsed := time.Since(start).Milliseconds()
	p.metrics.RecordComplete(elapsed)
	p.totalDone.Add(1)
	if err := p.queue.Ack(ctx, result.MessageID); err != nil {
		p.logger.Error("ack job", "job_id", job.ID, "error", err)
	}

	p.logger.Info("job completed", "job_id", job.ID, "total_done", p.totalDone.Load(), "elapsed_ms", elapsed)
	return nil
}

func (p *Processor) processJob(ctx context.Context, result *media.ClaimResult) error {
	job := result.Job

	// Update job status to running
	if err := p.updateJobRunning(ctx, job.ID); err != nil {
		return fmt.Errorf("update job running: %w", err)
	}

	// Create work directory
	workDir := filepath.Join(p.workDir, job.ID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Download raw file from storage
	rawPath := filepath.Join(workDir, "input")
	if err := p.downloadFile(ctx, job.RawObjectKey, rawPath); err != nil {
		return media.ClassifyStorageError(fmt.Errorf("download raw file: %w", err))
	}

	// Run ffprobe to get metadata
	probeResult, err := media.Probe(rawPath)
	if err != nil {
		p.logger.Warn("ffprobe failed, continuing without metadata", "error", err)
	}

	// Run FFmpeg to generate HLS renditions
	hlsDir := filepath.Join(workDir, "hls")
	if err := os.MkdirAll(hlsDir, 0755); err != nil {
		return fmt.Errorf("create hls dir: %w", err)
	}

	renditions := media.DefaultRenditions()
	cmd := media.BuildHLSCommand(rawPath, hlsDir, renditions)

	// Capture stderr for progress
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ffmpeg start: %w", err)
	}

	totalDuration := float64(0)
	if probeResult != nil {
		totalDuration = probeResult.DurationSeconds
	}

	// Read stderr line by line for progress reporting.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := stderr.Read(buf)
			if n > 0 {
				_ = p.parseAndUpdateProgress(ctx, job.ID, string(buf[:n]), totalDuration)
			}
			if readErr != nil {
				break
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		p.metrics.RecordFFmpegFailure()
		return media.ClassifyFFmpegError(fmt.Errorf("ffmpeg failed: %w", err))
	}

	// Upload HLS outputs to storage
	if err := p.uploadHLSOutputs(ctx, job.VideoID, hlsDir); err != nil {
		return media.ClassifyStorageError(fmt.Errorf("upload HLS outputs: %w", err))
	}

	// Generate and upload thumbnail/poster
	width, height := 0, 0
	duration := float64(0)
	if probeResult != nil {
		width = probeResult.Width
		height = probeResult.Height
		duration = probeResult.DurationSeconds
	}

	posterKey, thumbErr := p.generateAndUploadThumbnail(ctx, rawPath, job.VideoID, duration, workDir)
	if thumbErr != nil {
		// Non-fatal: log and continue without a thumbnail.
		p.logger.Warn("thumbnail generation failed", "job_id", job.ID, "error", thumbErr)
		posterKey = ""
	}

	if err := p.updateVideoReady(ctx, job.VideoID, width, height, duration, renditions, posterKey); err != nil {
		return fmt.Errorf("update video ready: %w", err)
	}
	if err := p.indexVideoReady(ctx, job.VideoID); err != nil {
		p.logger.Warn("index ready video failed", "video_id", job.VideoID, "error", err)
	}

	// ── DASH (non-fatal) ──────────────────────────────────────────────────────
	hasDash := false
	dashDir := filepath.Join(workDir, "dash")
	if err := os.MkdirAll(dashDir, 0755); err == nil {
		dashCmd := media.BuildDASHCommand(rawPath, dashDir, renditions)
		if err := dashCmd.Run(); err != nil {
			p.logger.Warn("DASH transcoding failed (non-fatal)", "job_id", job.ID, "error", err)
		} else if err := p.uploadDASHOutputs(ctx, job.VideoID, dashDir); err != nil {
			p.logger.Warn("DASH upload failed (non-fatal)", "job_id", job.ID, "error", err)
		} else {
			hasDash = true
		}
	}

	// ── Subtitles (non-fatal) ─────────────────────────────────────────────────
	hasSubtitles := false
	vttPath := filepath.Join(workDir, "subtitles.vtt")
	if err := media.ExtractSubtitles(rawPath, vttPath); err != nil {
		if !errors.Is(err, media.ErrNoSubtitleStream) {
			p.logger.Warn("subtitle extraction failed (non-fatal)", "job_id", job.ID, "error", err)
		}
	} else if err := p.uploadSubtitleVTT(ctx, job.VideoID, vttPath); err != nil {
		p.logger.Warn("subtitle upload failed (non-fatal)", "job_id", job.ID, "error", err)
	} else {
		hasSubtitles = true
	}

	if hasDash || hasSubtitles {
		if _, err := p.db.Exec(ctx,
			`UPDATE videos SET has_dash = $2, has_subtitles = $3 WHERE id = $1`,
			job.VideoID, hasDash, hasSubtitles,
		); err != nil {
			p.logger.Warn("update video dash/subtitle flags failed (non-fatal)", "job_id", job.ID, "error", err)
		}
	}

	// Mark job complete
	if err := p.updateJobComplete(ctx, job.ID); err != nil {
		return fmt.Errorf("update job complete: %w", err)
	}

	return nil
}

func (p *Processor) downloadFile(ctx context.Context, objectKey, destPath string) error {
	reader, err := p.storage.GetObject(ctx, "tpt-media", objectKey)
	if err != nil {
		return fmt.Errorf("get object: %w", err)
	}
	defer reader.Close()

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	_, err = f.ReadFrom(reader)
	return err
}

func (p *Processor) uploadHLSOutputs(ctx context.Context, videoID, hlsDir string) error {
	// Walk the hls directory and upload each file
	entries, err := os.ReadDir(hlsDir)
	if err != nil {
		return fmt.Errorf("read hls dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(hlsDir, entry.Name())
		objectKey := fmt.Sprintf("hls/%s/%s", videoID, entry.Name())

		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open hls file: %w", err)
		}

		info, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("stat hls file: %w", err)
		}

		contentType := "application/octet-stream"
		if filepath.Ext(entry.Name()) == ".m3u8" {
			contentType = "application/vnd.apple.mpegurl"
		} else if filepath.Ext(entry.Name()) == ".ts" {
			contentType = "video/MP2T"
		}

		if err := p.storage.PutObject(ctx, "tpt-media", objectKey, f, info.Size(), contentType); err != nil {
			f.Close()
			return fmt.Errorf("put hls object: %w", err)
		}
		f.Close()
	}

	return nil
}

// parseAndUpdateProgress scans a chunk of FFmpeg stderr for the latest timecode
// and writes the derived progress percentage to the database.
// totalDuration of 0 means unknown; in that case progress is not updated.
func (p *Processor) parseAndUpdateProgress(ctx context.Context, jobID, output string, totalDuration float64) error {
	if totalDuration <= 0 {
		return nil
	}
	currentSec := media.ParseFFmpegTimecode(output)
	if currentSec < 0 {
		return nil
	}
	pct := currentSec / totalDuration * 100
	if pct > 99 {
		pct = 99 // reserve 100 for the explicit completion update
	}
	_, _ = p.db.Exec(ctx,
		`UPDATE transcode_jobs SET progress_percent = GREATEST(progress_percent, $2) WHERE id = $1`,
		jobID, pct,
	)
	return nil
}

func (p *Processor) updateJobRunning(ctx context.Context, jobID string) error {
	_, err := p.db.Exec(ctx,
		`UPDATE transcode_jobs SET status = 'running', claimed_at = now(), started_at = now(), attempt = attempt + 1 WHERE id = $1`,
		jobID,
	)
	return err
}

func (p *Processor) updateJobFailed(ctx context.Context, jobID, errMsg string) {
	_, _ = p.db.Exec(ctx,
		`UPDATE transcode_jobs SET status = 'failed', error_message = $1 WHERE id = $2`,
		errMsg, jobID,
	)
}

func (p *Processor) updateJobComplete(ctx context.Context, jobID string) error {
	_, err := p.db.Exec(ctx,
		`UPDATE transcode_jobs SET status = 'complete', completed_at = now(), progress_percent = 100 WHERE id = $1`,
		jobID,
	)
	return err
}

func (p *Processor) generateAndUploadThumbnail(ctx context.Context, inputPath, videoID string, duration float64, workDir string) (string, error) {
	seekSec := duration / 4
	if seekSec < 1 {
		seekSec = 1
	}

	posterPath := filepath.Join(workDir, "poster.jpg")
	if err := media.GenerateThumbnail(inputPath, posterPath, seekSec); err != nil {
		return "", err
	}

	f, err := os.Open(posterPath)
	if err != nil {
		return "", fmt.Errorf("open poster: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat poster: %w", err)
	}

	objectKey := fmt.Sprintf("thumbnails/%s/poster.jpg", videoID)
	if err := p.storage.PutObject(ctx, "tpt-media", objectKey, f, info.Size(), "image/jpeg"); err != nil {
		return "", fmt.Errorf("upload poster: %w", err)
	}

	return objectKey, nil
}

func (p *Processor) updateVideoReady(ctx context.Context, videoID string, width, height int, duration float64, renditions []media.HLSRendition, posterKey string) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE videos SET status = 'ready', width = $2, height = $3, duration_seconds = $4,
		 poster_object_key = NULLIF($5, ''), published_at = now()
		 WHERE id = $1`,
		videoID, width, height, duration, posterKey,
	)
	if err != nil {
		return fmt.Errorf("update video: %w", err)
	}

	// Insert renditions
	for _, r := range renditions {
		key := fmt.Sprintf("hls/%s/%s.m3u8", videoID, r.Name)
		_, err = tx.Exec(ctx,
			`INSERT INTO video_renditions (video_id, name, width, height, bitrate, hls_manifest_object_key, status)
			 VALUES ($1, $2, $3, $4, $5, $6, 'ready')`,
			videoID, r.Name, r.Width, r.Height, r.Bitrate*1000, key,
		)
		if err != nil {
			return fmt.Errorf("insert rendition: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (p *Processor) indexVideoReady(ctx context.Context, videoID string) error {
	if p.search == nil {
		return nil
	}

	var id, title, description, ownerID, ownerDisplayName string
	var durationSeconds *int
	var viewCount, likeCount int64
	var createdAt, publishedAt *time.Time

	err := p.db.QueryRow(ctx, `
		SELECT v.id::text, v.title, coalesce(v.description, ''), v.owner_id::text, u.display_name,
		       v.duration_seconds, v.view_count, v.like_count, v.created_at, v.published_at
		FROM videos v
		JOIN users u ON u.id = v.owner_id
		WHERE v.id = $1 AND v.deleted_at IS NULL`, videoID).
		Scan(&id, &title, &description, &ownerID, &ownerDisplayName, &durationSeconds, &viewCount, &likeCount, &createdAt, &publishedAt)
	if err != nil {
		return fmt.Errorf("load video for search index: %w", err)
	}

	return p.search.IndexVideo(ctx, search.Video{
		ID:               id,
		Title:            title,
		Description:      description,
		OwnerID:          ownerID,
		OwnerDisplayName: ownerDisplayName,
		MediaType:        search.MediaTypeVOD,
		DurationSeconds:  durationSeconds,
		ViewCount:        viewCount,
		LikeCount:        likeCount,
		CreatedAt:        *createdAt,
		PublishedAt:      publishedAt,
	})
}

func (p *Processor) uploadDASHOutputs(ctx context.Context, videoID, dashDir string) error {
	entries, err := os.ReadDir(dashDir)
	if err != nil {
		return fmt.Errorf("read dash dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(dashDir, entry.Name())
		objectKey := fmt.Sprintf("dash/%s/%s", videoID, entry.Name())

		f, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("open dash file: %w", err)
		}
		info, err := f.Stat()
		if err != nil {
			f.Close()
			return fmt.Errorf("stat dash file: %w", err)
		}

		contentType := "application/octet-stream"
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".mpd":
			contentType = "application/dash+xml"
		case ".mp4":
			contentType = "video/mp4"
		case ".m4s":
			contentType = "video/iso.segment"
		}

		if err := p.storage.PutObject(ctx, "tpt-media", objectKey, f, info.Size(), contentType); err != nil {
			f.Close()
			return fmt.Errorf("put dash object: %w", err)
		}
		f.Close()
	}
	return nil
}

func (p *Processor) uploadSubtitleVTT(ctx context.Context, videoID, vttPath string) error {
	f, err := os.Open(vttPath)
	if err != nil {
		return fmt.Errorf("open vtt: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat vtt: %w", err)
	}

	key := fmt.Sprintf("subtitles/%s/default.vtt", videoID)
	return p.storage.PutObject(ctx, "tpt-media", key, f, info.Size(), "text/vtt")
}
