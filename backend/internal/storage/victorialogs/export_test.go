package victorialogs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/storage"
)

func TestExportStreamsMoreThanSearchPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if !strings.Contains(r.Form.Get("query"), "| limit 10000") {
			t.Errorf("missing bounded export limit: %s", r.Form.Get("query"))
		}
		for i := 0; i < 1500; i++ {
			fmt.Fprintf(w, "{\"_time\":\"2026-09-17T02:00:00Z\",\"_msg\":\"event-%d\"}\n", i)
		}
	}))
	defer server.Close()
	c, err := New(server.URL, 10*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	q := storage.Query{TenantID: "default", Start: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)}
	var out bytes.Buffer
	if err := c.Export(context.Background(), q, "ndjson", &out); err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(out.Bytes(), []byte("\n")); got != 1500 {
		t.Fatalf("NDJSON rows=%d", got)
	}
	out.Reset()
	if err := c.Export(context.Background(), q, "json", &out); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1500 {
		t.Fatalf("JSON rows=%d", len(rows))
	}
}

func TestExportCSVEscapesFormula(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "{\"_time\":\"2026-09-17T02:00:00Z\",\"_msg\":\"=HYPERLINK(1)\"}")
	}))
	defer server.Close()
	c, _ := New(server.URL, time.Second, nil)
	q := storage.Query{TenantID: "default", Start: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)}
	var out bytes.Buffer
	if err := c.Export(context.Background(), q, "csv", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "'=HYPERLINK(1)") {
		t.Fatalf("unsafe CSV: %s", out.String())
	}
}
