package ingestion

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	metricset "github.com/freezxp/syslogx/backend/internal/metrics"
	"github.com/freezxp/syslogx/backend/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
)

type fakeBackend struct {
	mu       sync.Mutex
	failures int
	calls    int
	logs     []domain.LogEntry
}

func (f *fakeBackend) Append(_ context.Context, logs []domain.LogEntry) (storage.AppendResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failures {
		return storage.AppendResult{}, storage.ErrUnavailable
	}
	f.logs = append(f.logs, logs...)
	return storage.AppendResult{Accepted: len(logs), Bytes: len(logs) * 100}, nil
}
func (*fakeBackend) Check(context.Context) storage.Health { return storage.Health{Healthy: true} }
func (*fakeBackend) Capabilities() storage.Capabilities   { return storage.Capabilities{Backend: "fake"} }

func TestPipelineDrainsAndRetries(t *testing.T) {
	cfg := config.Default().Ingestion
	cfg.Queue.MaxEvents = 10
	cfg.Queue.MaxBytes = 1024
	cfg.Queue.EnqueueTimeout = time.Millisecond
	cfg.Batch.MaxEvents = 2
	cfg.Batch.MaxBytes = 1024
	cfg.Batch.MaxAge = time.Hour
	cfg.Retry.MaxAttempts = 3
	cfg.Retry.InitialBackoff = time.Millisecond
	cfg.Retry.MaxBackoff = time.Millisecond
	cfg.Sources = []config.SourceConfig{{ID: "s", Parser: "auto", Timezone: "UTC"}}
	backend := &fakeBackend{failures: 1}
	reg := prometheus.NewRegistry()
	m := metricset.New(reg)
	accepting := &atomic.Bool{}
	p, err := NewPipeline(cfg, backend, m, accepting, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background(), 2)
	for _, msg := range []string{"one", "two"} {
		if err := p.Submit(context.Background(), domain.Frame{Payload: []byte(msg), ReceivedAt: time.Now(), SourceID: "s", TenantID: "default", Protocol: "syslog_udp"}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if backend.calls != 2 || len(backend.logs) != 2 {
		t.Fatalf("calls=%d logs=%d", backend.calls, len(backend.logs))
	}
}

func TestPipelineRejectsOversizedFrame(t *testing.T) {
	cfg := config.Default().Ingestion
	cfg.Queue.MaxEvents = 1
	cfg.Queue.MaxBytes = 2
	cfg.Queue.EnqueueTimeout = time.Millisecond
	cfg.Sources = nil
	backend := &fakeBackend{}
	p, err := NewPipeline(cfg, backend, metricset.New(prometheus.NewRegistry()), &atomic.Bool{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background(), 1)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = p.Stop(ctx)
	}()
	if err := p.Submit(context.Background(), domain.Frame{Payload: []byte("123")}); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected full, got %v", err)
	}
}
