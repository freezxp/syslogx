package syslog

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/ingestion"
	metricset "github.com/freezxp/syslogx/backend/internal/metrics"
	"github.com/freezxp/syslogx/backend/internal/storage"
	"github.com/prometheus/client_golang/prometheus"
)

type captureBackend struct {
	once     sync.Once
	received chan domain.LogEntry
}

func (b *captureBackend) Append(_ context.Context, logs []domain.LogEntry) (storage.AppendResult, error) {
	for _, entry := range logs {
		e := entry
		b.once.Do(func() { b.received <- e })
	}
	return storage.AppendResult{Accepted: len(logs)}, nil
}
func (*captureBackend) Check(context.Context) storage.Health { return storage.Health{Healthy: true} }
func (*captureBackend) Capabilities() storage.Capabilities {
	return storage.Capabilities{Backend: "capture"}
}

func TestUDPToStorage(t *testing.T) {
	probe, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.LocalAddr().String()
	_ = probe.Close()
	entry := runTransportTest(t, config.SourceConfig{ID: "udp", Name: "test", Protocol: "udp", Address: address, Parser: "auto", Enabled: true, MaxMessageBytes: 4096, TenantID: "default", Timezone: "UTC"}, func() {
		conn, err := net.Dial("udp", address)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_, err = conn.Write([]byte(`<34>Sep 14 14:30:00 host app[7]: hello udp`))
		if err != nil {
			t.Fatal(err)
		}
	})
	if entry.Message != "hello udp" || entry.Hostname != "host" || entry.Protocol != "syslog_udp" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func TestTCPToStorage(t *testing.T) {
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := probe.Addr().String()
	_ = probe.Close()
	entry := runTransportTest(t, config.SourceConfig{ID: "tcp", Name: "test", Protocol: "tcp", Address: address, Parser: "auto", Framing: "auto", Enabled: true, MaxMessageBytes: 4096, MaxConnections: 4, TenantID: "default", Timezone: "UTC"}, func() {
		conn, err := net.Dial("tcp", address)
		if err != nil {
			t.Fatal(err)
		}
		_, err = conn.Write([]byte(`53 <34>1 2026-09-14T14:30:00Z host app 7 msg - hello tcp`))
		_ = conn.Close()
		if err != nil {
			t.Fatal(err)
		}
	})
	if entry.Message != "hello tcp" || entry.MessageID != "msg" || entry.Protocol != "syslog_tcp" {
		t.Fatalf("unexpected entry: %#v", entry)
	}
}

func runTransportTest(t *testing.T, source config.SourceConfig, send func()) domain.LogEntry {
	t.Helper()
	backend := &captureBackend{received: make(chan domain.LogEntry, 1)}
	cfg := config.Default().Ingestion
	cfg.Sources = []config.SourceConfig{source}
	cfg.Queue.MaxEvents = 10
	cfg.Queue.MaxBytes = 1 << 20
	cfg.Queue.EnqueueTimeout = time.Second
	cfg.Batch.MaxEvents = 1
	cfg.Batch.MaxBytes = 1 << 20
	cfg.Batch.MaxAge = 10 * time.Millisecond
	m := metricset.New(prometheus.NewRegistry())
	accepting := &atomic.Bool{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pipeline, err := ingestion.NewPipeline(cfg, backend, m, accepting, logger)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pipeline.Start(ctx, 2)
	manager := NewManager(cfg.Sources, pipeline, m, logger)
	if err := manager.Start(ctx); err != nil {
		t.Fatal(err)
	}
	send()
	var entry domain.LogEntry
	select {
	case entry = <-backend.received:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for stored event")
	}
	if err := manager.Close(); err != nil {
		t.Fatal(err)
	}
	stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
	defer stopCancel()
	if err := pipeline.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	return entry
}
