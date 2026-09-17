package victorialogs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/storage"
)

func TestNativeAggregates(t *testing.T) {
	queries := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		q := r.Form.Get("query")
		queries = append(queries, q)
		if !strings.Contains(q, "tenant_id:") {
			t.Errorf("missing tenant scope: %s", q)
		}
		switch {
		case strings.Contains(q, "stats count() as total"):
			fmt.Fprintln(w, "{\"total\":\"5000\"}")
		case strings.Contains(q, "stats by (_time:"):
			fmt.Fprintln(w, "{\"_time\":\"2026-09-17T02:00:00Z\",\"hits\":\"4900\"}")
		case strings.Contains(q, "stats by (severity_name)"):
			fmt.Fprintln(w, "{\"severity_name\":\"error\",\"hits\":\"4900\"}")
		case strings.Contains(q, "stats by (hostname)"):
			fmt.Fprintln(w, "{\"hostname\":\"fw01\",\"hits\":\"5000\"}")
		case strings.Contains(q, "stats by (app_name)"):
			fmt.Fprintln(w, "{\"app_name\":\"vpn\",\"hits\":\"5000\"}")
		case strings.Contains(q, "field_names"):
			fmt.Fprintln(w, "{\"name\":\"hostname\",\"hits\":\"5000\"}")
		case strings.Contains(q, "field_values hostname"):
			fmt.Fprintln(w, "{\"hits\":\"4999\"}")
			fmt.Fprintln(w, "{\"hostname\":\"fw01\",\"hits\":\"5000\"}")
		default:
			t.Errorf("unexpected query: %s", q)
		}
	}))
	defer server.Close()
	c, err := New(server.URL, time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	q := storage.Query{TenantID: "default", Start: time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)}
	stats, err := c.Stats(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Total != 5000 || len(stats.Volume) != 1 || stats.Volume[0].Count != 4900 || len(stats.Severities) != 1 || stats.Severities[0].Count != 4900 {
		t.Fatalf("stats: %+v", stats)
	}
	names, err := c.FieldNames(context.Background(), q)
	if err != nil || len(names) != 1 || names[0].Count != 5000 {
		t.Fatalf("names=%+v err=%v", names, err)
	}
	values, err := c.FieldValues(context.Background(), q, "hostname", 10)
	if err != nil || len(values) != 1 || values[0].Count != 5000 {
		t.Fatalf("values=%+v err=%v", values, err)
	}
	before := len(queries)
	if _, err := c.FieldValues(context.Background(), q, "host|delete", 10); err == nil {
		t.Fatal("unsafe field accepted")
	}
	if len(queries) != before {
		t.Fatal("unsafe field reached backend")
	}
}
