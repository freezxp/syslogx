package victorialogs

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
)

func TestAppendMapping(t *testing.T) {
	var row map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/insert/jsonline" || r.Header.Get("VL-Msg-Field") != "message" || r.Header.Get("VL-Stream-Fields") != "tenant_id,hostname" {
			t.Errorf("unexpected request: %s %#v", r.URL.Path, r.Header)
		}
		if err := json.NewDecoder(r.Body).Decode(&row); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second, []string{"tenant_id", "hostname"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	result, err := c.Append(context.Background(), []domain.LogEntry{{ID: "id", TenantID: "default", Timestamp: now, ReceivedAt: now, Message: "hello", Protocol: "syslog_udp", Format: "rfc3164", SourceID: "source", SourceType: "syslog", Fields: map[string]any{"vendor": "acme"}, Labels: map[string]string{"env": "test"}, ParseStatus: domain.ParseStatusParsed, SchemaVersion: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Accepted != 1 || result.Bytes == 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if row["message"] != "hello" || row["vendor"] != "acme" || row["label.env"] != "test" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, nil)
	if h := c.Check(context.Background()); !h.Healthy {
		t.Fatalf("unhealthy: %#v", h)
	}
}
