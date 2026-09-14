package query

import (
	"github.com/freezxp/syslogx/backend/internal/storage"
	"strings"
	"testing"
	"time"
)

func TestLogsQLEscapes(t *testing.T) {
	q := storage.Query{TenantID: "default", Start: time.Now().Add(-time.Hour), End: time.Now(), Limit: 10, Filters: []storage.Filter{{Field: "hostname", Op: "eq", Value: "x\" OR *"}}}
	got, err := LogsQL(q)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `hostname:"x\" OR *"`) {
		t.Fatalf("unsafe output %s", got)
	}
}
func TestRejectsFieldInjection(t *testing.T) {
	q := storage.Query{Start: time.Now().Add(-time.Hour), End: time.Now(), Limit: 10, Filters: []storage.Filter{{Field: "x | drop", Op: "eq", Value: "a"}}}
	if _, err := LogsQL(q); err == nil {
		t.Fatal("expected error")
	}
}
