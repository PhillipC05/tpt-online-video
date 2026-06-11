package processor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tpt-online-video/packages/media"
	"github.com/tpt-online-video/packages/storage"
)

type Processor struct {
	logger   *slog.Logger
	db       *pgxpool.Pool
	redis    *redis.Client
	storage  storage.Provider
	queue    *media.Queue
	workDir  string
}

func New(logger *slog.Logger, db *pgxpool.Pool, redis *redis.Client, store storage.Provider, queue *media.Queue, workDir string) *Processor {
	return &Processor{
		logger:  logger,
		db:      db,
		redis:   redis,
		storage: store,
		queue:   queue,
		workDir: workDir,
	}
}

// Run starts the worker loop, blocking until the context is cancelled.
func (p *Processor) Run(ctx context.Context, concurrency int) {
	p.logger.Info("worker processor started", "concurrency", concurrency)

	// Use a semaphore-like pattern for concurrency
	sem := make(chan struct{}, concurrency)

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("worker processor stopping")
			return
		case sem <- struct{}{}:
			go func() {
				defer func() { <-sem }()
				if err := p.processNext(ctx); err != nil {
					if err != context.Canceled {
						p.logger.Error("process next job", "error", err)
					}
				}
			}()
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

	p.logger.Info("processing job", "job_id", result.Job.ID, "video_id", result.Job.VideoID)

	if err := p.processJob(ctx, result); err != nil {
		p.logger.Error("job failed", "job_id", result.Job.ID, "error", err)
		// Update job status in DB to failed
		p.updateJobFailed(ctx, result.Job.ID, err.Error())
		// Nack for retry
		_ = p.queue.Nack(ctx, result.MessageID)
		return err
	}

	// Ack on success
	if err := p.queue.Ack(ctx, result.MessageID); err != nil {
		p.logger.Error("ack job", "job_id", result.Job.ID, "error", err)
	}

	p.logger.Info("job completed", "job_id", result.Job.ID)
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
		return fmt.Errorf("download raw file: %w", err)
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

	// Read stderr line by line for progress
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				_ = p.parseAndUpdateProgress(ctx, job.ID, string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	// Upload HLS outputs to storage
	if err := p.uploadHLSOutputs(ctx, job.VideoID, hlsDir); err != nil {
		return fmt.Errorf("upload HLS outputs: %w", err)
	}

	// Update video status to ready
	width, height := 0, 0
	duration := float64(0)
	if probeResult != nil {
		width = probeResult.Width
		height = probeResult.Height
		duration = probeResult.DurationSeconds
	}

	if err := p.updateVideoReady(ctx, job.VideoID, width, height, duration, renditions); err != nil {
		return fmt.Errorf("update video ready: %w", err)
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

func (p *Processor) parseAndUpdateProgress(ctx context.Context, jobID, output string) error {
	// Parse FFmpeg stderr for time=... to calculate progress
	// Simple: scan for "time=" pattern
	var timeStr string
	if _, err := fmt.Sscanf(output, " time=%s", &timeStr); err == nil && timeStr != "" {
		// Parse HH:MM:SS.MS format
		var h, m int
		var s float64
		if _, err := fmt.Sscanf(timeStr, "%d:%d:%f", &h, &m, &s); err == nil {
			currentSec := float64(h*3600+m*60) + s
			_ = currentSec

			// Update progress (we don't know total duration yet, so use a simple approach)
			// For now, just update that we're processing
			_, _ = p.db.Exec(ctx,
				`UPDATE transcode_jobs SET progress_percent = GREATEST(progress_percent, 0.01) WHERE id = $1`,
				jobID,
			)
		}
	}
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

func (p *Processor) updateVideoReady(ctx context.Context, videoID string, width, height int, duration float64, renditions []media.HLSRendition) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE videos SET status = 'ready', width = $2, height = $3, duration_seconds = $4, published_at = now()
		 WHERE id = $1`,
		videoID, width, height, duration,
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