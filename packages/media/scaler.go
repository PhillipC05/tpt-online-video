package media

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// ScalerConfig controls the scaling policy for the worker pool.
type ScalerConfig struct {
	// MinWorkers is the lower bound on desired concurrency.
	MinWorkers int
	// MaxWorkers is the upper bound on desired concurrency.
	MaxWorkers int

	// ScaleUpQueueDepth triggers a scale-up when pending jobs exceed this value.
	ScaleUpQueueDepth int64
	// ScaleDownQueueDepth triggers a scale-down when pending jobs fall below this value.
	ScaleDownQueueDepth int64

	// ScaleUpBusyPct triggers a scale-up when worker utilization exceeds this
	// fraction (0–1). Utilization is sampled as busy/desired workers.
	ScaleUpBusyPct float64
	// ScaleDownBusyPct triggers a scale-down when worker utilization falls below
	// this fraction (0–1).
	ScaleDownBusyPct float64

	// EvalInterval is how often the scaler evaluates the policy.
	EvalInterval time.Duration
	// CooldownUp is the minimum time between successive scale-up events.
	CooldownUp time.Duration
	// CooldownDown is the minimum time between successive scale-down events.
	CooldownDown time.Duration
}

// DefaultScalerConfig returns a conservative starting configuration.
func DefaultScalerConfig() ScalerConfig {
	return ScalerConfig{
		MinWorkers:          1,
		MaxWorkers:          8,
		ScaleUpQueueDepth:   5,
		ScaleDownQueueDepth: 1,
		ScaleUpBusyPct:      0.80,
		ScaleDownBusyPct:    0.30,
		EvalInterval:        15 * time.Second,
		CooldownUp:          30 * time.Second,
		CooldownDown:        60 * time.Second,
	}
}

// ScalingController monitors queue depth and worker utilization, then adjusts
// the desired worker concurrency within the configured min/max bounds.
//
// Utilization is measured as the ratio of busy worker slots to desired slots,
// which reflects CPU load for a CPU-bound transcoding workload more accurately
// than raw system CPU percentage.
type ScalingController struct {
	queue   *Queue
	config  ScalerConfig
	logger  *slog.Logger
	desired atomic.Int32

	mu       sync.Mutex
	lastUp   time.Time
	lastDown time.Time

	sampleMu    sync.Mutex
	busySamples []float64 // rolling window (last 10 samples)
}

// NewScalingController creates a ScalingController with desired initialised to
// config.MinWorkers. If logger is nil, a no-op logger is used.
func NewScalingController(queue *Queue, config ScalerConfig, logger *slog.Logger) *ScalingController {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	sc := &ScalingController{
		queue:  queue,
		config: config,
		logger: logger,
	}
	sc.desired.Store(int32(config.MinWorkers))
	return sc
}

// Config returns the ScalerConfig this controller was created with.
func (sc *ScalingController) Config() ScalerConfig { return sc.config }

// Desired returns the current desired worker concurrency.
func (sc *ScalingController) Desired() int {
	return int(sc.desired.Load())
}

// RecordUtilization records one utilization sample. busy is the number of
// worker goroutines currently processing a job; total is the current desired
// concurrency. Safe for concurrent use.
func (sc *ScalingController) RecordUtilization(busy, total int) {
	if total <= 0 {
		return
	}
	sample := float64(busy) / float64(total)
	sc.sampleMu.Lock()
	defer sc.sampleMu.Unlock()
	sc.busySamples = append(sc.busySamples, sample)
	if len(sc.busySamples) > 10 {
		sc.busySamples = sc.busySamples[len(sc.busySamples)-10:]
	}
}

func (sc *ScalingController) avgUtilization() float64 {
	sc.sampleMu.Lock()
	defer sc.sampleMu.Unlock()
	if len(sc.busySamples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range sc.busySamples {
		sum += s
	}
	return sum / float64(len(sc.busySamples))
}

// Run evaluates the scaling policy on each EvalInterval tick until ctx is done.
func (sc *ScalingController) Run(ctx context.Context) {
	ticker := time.NewTicker(sc.config.EvalInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sc.evaluate(ctx)
		}
	}
}

func (sc *ScalingController) evaluate(ctx context.Context) {
	metrics, err := sc.queue.Metrics(ctx)
	if err != nil {
		sc.logger.Warn("scaler: failed to get queue metrics", "error", err)
		return
	}

	utilization := sc.avgUtilization()
	current := sc.Desired()
	now := time.Now()

	// Effective queue depth = undelivered jobs + jobs being retried (pending).
	queueDepth := metrics.UndeliveredCount + metrics.PendingCount

	sc.logger.Debug("scaler evaluation",
		"queue_depth", queueDepth,
		"undelivered", metrics.UndeliveredCount,
		"pending", metrics.PendingCount,
		"dlq", metrics.DLQLength,
		"utilization_pct", int(utilization*100),
		"desired", current,
	)

	sc.mu.Lock()
	defer sc.mu.Unlock()

	// Scale-up: queue too deep or workers fully saturated.
	if current < sc.config.MaxWorkers && now.Sub(sc.lastUp) >= sc.config.CooldownUp {
		if queueDepth > sc.config.ScaleUpQueueDepth || utilization > sc.config.ScaleUpBusyPct {
			next := min(current+1, sc.config.MaxWorkers)
			sc.desired.Store(int32(next))
			sc.lastUp = now
			sc.logger.Info("scaler: scaling up",
				"from", current, "to", next,
				"queue_depth", queueDepth,
				"utilization_pct", int(utilization*100),
			)
			return
		}
	}

	// Scale-down: queue drained and workers mostly idle.
	if current > sc.config.MinWorkers && now.Sub(sc.lastDown) >= sc.config.CooldownDown {
		if queueDepth < sc.config.ScaleDownQueueDepth && utilization < sc.config.ScaleDownBusyPct {
			next := max(current-1, sc.config.MinWorkers)
			sc.desired.Store(int32(next))
			sc.lastDown = now
			sc.logger.Info("scaler: scaling down",
				"from", current, "to", next,
				"queue_depth", queueDepth,
				"utilization_pct", int(utilization*100),
			)
		}
	}
}
