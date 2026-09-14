package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/storage"
)

type testBackend struct{ healthy bool }

func (*testBackend) Append(context.Context, []domain.LogEntry) (storage.AppendResult, error) {
	return storage.AppendResult{}, nil
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
