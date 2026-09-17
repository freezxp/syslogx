package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/storage"
)

type repeatTailBackend struct {
	*testBackend
	storage.Querier
	calls atomic.Int32
}

func (b *repeatTailBackend) Query(_ context.Context, q storage.Query) (storage.QueryResult, error) {
	b.calls.Add(1)
	if q.End.Sub(q.Start) != 30*time.Second {
		return storage.QueryResult{}, nil
	}
	return storage.QueryResult{Data: []map[string]any{{"id": "tail-1", "_time": "2026-09-17T05:00:00Z", "_msg": "once"}}}, nil
}

func TestTailDeduplicatesOverlappingPolls(t *testing.T) {
	accepting := &atomic.Bool{}
	accepting.Store(true)
	backend := &repeatTailBackend{testBackend: &testBackend{healthy: true}}
	server := New(config.Default().Server.HTTP, backend, nil, 0, accepting, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/tail?query=once", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, req)
	body := rec.Body.String()
	if backend.calls.Load() < 2 {
		t.Fatalf("polls=%d", backend.calls.Load())
	}
	if got := strings.Count(body, "data: "); got != 1 {
		t.Fatalf("events=%d body=%s", got, body)
	}
	if !strings.Contains(body, ": heartbeat") {
		t.Fatalf("missing heartbeat: %s", body)
	}
}

func TestTailRejectsOversizedQuery(t *testing.T) {
	accepting := &atomic.Bool{}
	server := New(config.Default().Server.HTTP, &repeatTailBackend{testBackend: &testBackend{healthy: true}}, nil, 0, accepting, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rec := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/logs/tail?query="+strings.Repeat("x", 4097), nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid query, got %d", rec.Code)
	}
}
