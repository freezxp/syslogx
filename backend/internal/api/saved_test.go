package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/controlstore"
)

type fakeSavedStore struct {
	mu    sync.Mutex
	items map[string]controlstore.SavedSearch
}

func (m *fakeSavedStore) ListSavedSearches(_ context.Context, owner string) ([]controlstore.SavedSearch, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []controlstore.SavedSearch{}
	for _, v := range m.items {
		if v.CreatedBy == owner {
			out = append(out, v)
		}
	}
	return out, nil
}
func (m *fakeSavedStore) CreateSavedSearch(_ context.Context, v controlstore.SavedSearch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[v.ID] = v
	return nil
}
func (m *fakeSavedStore) DeleteSavedSearch(_ context.Context, owner, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.items[id]
	if ok && v.CreatedBy == owner {
		delete(m.items, id)
		return true, nil
	}
	return false, nil
}

func TestSavedSearchUsesPersistentStoreAcrossServers(t *testing.T) {
	store := &fakeSavedStore{items: map[string]controlstore.SavedSearch{}}
	newServer := func() *Server {
		accepting := &atomic.Bool{}
		accepting.Store(true)
		return New(config.Default().Server.HTTP, &testBackend{healthy: true}, nil, 0, accepting, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{SavedSearches: store})
	}
	call := func(s *Server, method, path, body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		s.http.Handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		return rec
	}
	first := newServer()
	created := call(first, http.MethodPost, "/api/v1/saved-searches", `{"name":"Critical events","query":"severity:error"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var saved controlstore.SavedSearch
	if err := json.Unmarshal(created.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" || saved.CreatedBy != "development" {
		t.Fatalf("saved: %+v", saved)
	}
	second := newServer()
	listed := call(second, http.MethodGet, "/api/v1/saved-searches", "")
	if listed.Code != http.StatusOK || !strings.Contains(listed.Body.String(), saved.ID) {
		t.Fatalf("list: %d %s", listed.Code, listed.Body.String())
	}
	deleted := call(second, http.MethodDelete, "/api/v1/saved-searches/"+saved.ID, "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", deleted.Code)
	}
	missing := call(first, http.MethodDelete, "/api/v1/saved-searches/"+saved.ID, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", missing.Code)
	}
}

func TestSourcesReflectConfiguration(t *testing.T) {
	accepting := &atomic.Bool{}
	accepting.Store(true)
	s := New(config.Default().Server.HTTP, &testBackend{healthy: true}, nil, 0, accepting, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), Options{Sources: []config.SourceConfig{{ID: "udp-test", Name: "Test UDP", Protocol: "udp", Address: ":1514", Parser: "syslog", Enabled: true}}})
	rec := httptest.NewRecorder()
	s.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/sources", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"udp-test"`) || !strings.Contains(rec.Body.String(), `"status":"running"`) {
		t.Fatalf("sources: %d %s", rec.Code, rec.Body.String())
	}
}
