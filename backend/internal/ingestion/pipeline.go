package ingestion

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/metrics"
	"github.com/freezxp/syslogx/backend/internal/normalization"
	"github.com/freezxp/syslogx/backend/internal/parser"
	"github.com/freezxp/syslogx/backend/internal/storage"
)

type Pipeline struct {
	cfg         config.IngestionConfig
	queue       *frameQueue
	parsed      chan domain.LogEntry
	backend     storage.Backend
	parsers     *parser.Registry
	metrics     *metrics.Metrics
	logger      *slog.Logger
	accepting   *atomic.Bool
	cancel      context.CancelFunc
	done        chan struct{}
	workers     sync.WaitGroup
	locations   map[string]*time.Location
	parserNames map[string]string
	backendName string
}

func NewPipeline(cfg config.IngestionConfig, backend storage.Backend, m *metrics.Metrics, accepting *atomic.Bool, logger *slog.Logger) (*Pipeline, error) {
	locations := make(map[string]*time.Location, len(cfg.Sources))
	parserNames := make(map[string]string, len(cfg.Sources))
	for _, source := range cfg.Sources {
		loc, err := time.LoadLocation(source.Timezone)
		if err != nil {
			return nil, err
		}
		locations[source.ID] = loc
		parserNames[source.ID] = source.Parser
	}
	return &Pipeline{cfg: cfg, queue: newFrameQueue(cfg.Queue.MaxEvents, cfg.Queue.MaxBytes), parsed: make(chan domain.LogEntry, 64), backend: backend, parsers: parser.NewRegistry(), metrics: m, logger: logger, accepting: accepting, done: make(chan struct{}), locations: locations, parserNames: parserNames, backendName: backend.Capabilities().Backend}, nil
}

func (p *Pipeline) Start(parent context.Context, workers int) {
	if workers < 1 {
		workers = 1
	}
	ctx, cancel := context.WithCancel(parent)
	p.cancel = cancel
	p.accepting.Store(true)
	for range workers {
		p.workers.Add(1)
		go p.parseWorker(ctx)
	}
	go func() { p.workers.Wait(); close(p.parsed) }()
	go p.batchWorker(ctx)
}

func (p *Pipeline) Submit(ctx context.Context, frame domain.Frame) error {
	p.metrics.Received.WithLabelValues(frame.Protocol).Inc()
	p.metrics.BytesReceived.WithLabelValues(frame.Protocol).Add(float64(len(frame.Payload)))
	if !p.accepting.Load() {
		p.metrics.Dropped.WithLabelValues("admission", "draining").Inc()
		return ErrQueueClosed
	}
	err := p.queue.enqueue(ctx, frame, p.cfg.Queue.EnqueueTimeout)
	if err != nil {
		reason := "full"
		if errors.Is(err, ErrQueueClosed) {
			reason = "closed"
		}
		p.metrics.Dropped.WithLabelValues("admission", reason).Inc()
		return err
	}
	p.metrics.Accepted.WithLabelValues(frame.Protocol).Inc()
	p.updateQueueMetrics()
	return nil
}

func (p *Pipeline) Reject(protocol, reason string, bytes int) {
	p.metrics.Received.WithLabelValues(protocol).Inc()
	p.metrics.BytesReceived.WithLabelValues(protocol).Add(float64(bytes))
	p.metrics.Dropped.WithLabelValues("admission", reason).Inc()
}

func (p *Pipeline) Stop(ctx context.Context) error {
	p.accepting.Store(false)
	p.queue.close()
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		if p.cancel != nil {
			p.cancel()
		}
		<-p.done
		return ctx.Err()
	}
}

func (p *Pipeline) parseWorker(ctx context.Context) {
	defer p.workers.Done()
	for {
		item, err := p.queue.dequeue(ctx)
		if err != nil {
			return
		}
		p.updateQueueMetrics()
		loc := p.locations[item.frame.SourceID]
		parserName := p.parserNames[item.frame.SourceID]
		if parserName == "" {
			parserName = "auto"
		}
		event, err := p.parsers.Parse(ctx, parserName, item.frame.Payload, item.frame.ReceivedAt, loc)
		if err != nil {
			p.metrics.ParseErrors.WithLabelValues(parserName, "invalid").Inc()
			p.metrics.Dropped.WithLabelValues("parser", "invalid").Inc()
			continue
		}
		entry, err := normalization.Normalize(item.frame, event)
		if err != nil {
			p.metrics.Dropped.WithLabelValues("normalization", "internal").Inc()
			p.logger.Error("normalize log", "error", err)
			continue
		}
		p.metrics.Parsed.WithLabelValues(entry.Format).Inc()
		select {
		case p.parsed <- entry:
		case <-ctx.Done():
			p.metrics.Dropped.WithLabelValues("normalization", "shutdown").Inc()
			return
		}
	}
}

func (p *Pipeline) batchWorker(ctx context.Context) {
	defer close(p.done)
	timer := time.NewTimer(p.cfg.Batch.MaxAge)
	defer timer.Stop()
	batch := make([]domain.LogEntry, 0, p.cfg.Batch.MaxEvents)
	var batchBytes int64
	flush := func() {
		if len(batch) == 0 {
			return
		}
		p.appendWithRetry(ctx, batch)
		batch = make([]domain.LogEntry, 0, p.cfg.Batch.MaxEvents)
		batchBytes = 0
	}
	for {
		select {
		case entry, ok := <-p.parsed:
			if !ok {
				flush()
				return
			}
			size := int64(len(entry.RawMessage) + len(entry.Message))
			if len(batch) > 0 && (len(batch) >= p.cfg.Batch.MaxEvents || batchBytes+size > p.cfg.Batch.MaxBytes) {
				flush()
				resetTimer(timer, p.cfg.Batch.MaxAge)
			}
			batch = append(batch, entry)
			batchBytes += size
			if len(batch) >= p.cfg.Batch.MaxEvents || batchBytes >= p.cfg.Batch.MaxBytes {
				flush()
				resetTimer(timer, p.cfg.Batch.MaxAge)
			}
		case <-timer.C:
			flush()
			timer.Reset(p.cfg.Batch.MaxAge)
		case <-ctx.Done():
			for entry := range p.parsed {
				batch = append(batch, entry)
			}
			flush()
			return
		}
	}
}

func resetTimer(timer *time.Timer, d time.Duration) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(d)
}

func (p *Pipeline) appendWithRetry(ctx context.Context, batch []domain.LogEntry) {
	backoff := p.cfg.Retry.InitialBackoff
	for attempt := 1; attempt <= p.cfg.Retry.MaxAttempts; attempt++ {
		start := time.Now()
		result, err := p.backend.Append(ctx, batch)
		outcome := "success"
		if err != nil {
			outcome = "error"
		}
		p.metrics.StorageDuration.WithLabelValues(p.backendName, outcome).Observe(time.Since(start).Seconds())
		if err == nil {
			p.metrics.Stored.WithLabelValues(p.backendName).Add(float64(result.Accepted))
			p.metrics.BytesStored.Add(float64(result.Bytes))
			return
		}
		if attempt == p.cfg.Retry.MaxAttempts || ctx.Err() != nil {
			p.metrics.Dropped.WithLabelValues("storage", "retry_exhausted").Add(float64(len(batch)))
			p.logger.Error("storage append exhausted", "events", len(batch), "attempts", attempt, "error", err)
			return
		}
		p.metrics.Retried.WithLabelValues(p.backendName).Inc()
		wait := backoff + time.Duration(rand.Int64N(max(1, int64(backoff/2))))
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			continue
		}
		backoff = min(backoff*2, p.cfg.Retry.MaxBackoff)
	}
}

func (p *Pipeline) updateQueueMetrics() {
	events, bytes := p.queue.usage()
	p.metrics.QueueEvents.Set(float64(events))
	p.metrics.QueueBytes.Set(float64(bytes))
}
