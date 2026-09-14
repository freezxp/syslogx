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

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/storage"
)

type testBackend struct {
	healthy  bool
	appended []domain.LogEntry
}

func (b *testBackend) Append(_ context.Context, logs []domain.LogEntry) (storage.AppendResult, error) {
	b.appended = append(b.appended, logs...)
	return storage.AppendResult{Accepted: len(logs)}, nil
}

func TestJSONIngestForms(t *testing.T) {
	for _, tc := range []struct {
		name, contentType, body string
		want                    int
	}{{"single", "application/json", `{"message":"one","hostname":"web01","custom":42}`, 1}, {"array", "application/json", `[{"message":"one"},{"message":"two"}]`, 2}, {"ndjson", "application/x-ndjson", "{\"message\":\"one\"}\n{\"message\":\"two\"}\n", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			a := &atomic.Bool{}
			a.Store(true)
			b := &testBackend{healthy: true}
			s := New(config.Default().Server.HTTP, b, nil, 0, a, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest(http.MethodPost, "/api/v1/ingest", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rec := httptest.NewRecorder()
			s.http.Handler.ServeHTTP(rec, req)
			if rec.Code != 202 {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if len(b.appended) != tc.want {
				t.Fatalf("appended=%d want=%d", len(b.appended), tc.want)
			}
		})
	}
}
func (b *testBackend) Check(context.Context) storage.Health {
	return storage.Health{Healthy: b.healthy}
}
func (*testBackend) Capabilities() storage.Capabilities { return storage.Capabilities{Backend: "test"} }

func TestReady(t *testing.T) {
	for _, tc := range []struct {
		name               string
		accepting, healthy bool
		want               int
	}{{"ready", true, true, 200}, {"draining", false, true, 503}, {"storage unavailable", true, false, 503}} {
		t.Run(tc.name, func(t *testing.T) {
			a := &atomic.Bool{}
			a.Store(tc.accepting)
			s := New(config.Default().Server.HTTP, &testBackend{healthy: tc.healthy}, nil, 0, a, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			rec := httptest.NewRecorder()
			s.http.Handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("got %d want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestCapabilities(t *testing.T) {
	a := &atomic.Bool{}
	s := New(config.Default().Server.HTTP, &testBackend{healthy: true}, nil, 0, a, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
}
