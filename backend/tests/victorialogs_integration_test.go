package tests

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/storage/victorialogs"
)

func TestVictoriaLogsRoundTrip(t *testing.T) {
	endpoint := os.Getenv("SYSLOGX_TEST_VICTORIALOGS_URL")
	if endpoint == "" {
		t.Skip("set SYSLOGX_TEST_VICTORIALOGS_URL to run the real-backend integration test")
	}
	c, err := victorialogs.New(endpoint, 5*time.Second, []string{"tenant_id", "source_type", "hostname", "app_name"})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("integration-%d", now.UnixNano())
	entry := domain.LogEntry{ID: id, TenantID: "default", Timestamp: now, ReceivedAt: now, Message: "syslogx integration " + id, Hostname: "test-host", Protocol: "syslog_udp", Format: "rfc5424", SourceID: "integration", SourceType: "syslog", Fields: map[string]any{}, Labels: map[string]string{}, ParseStatus: domain.ParseStatusParsed, SchemaVersion: domain.SchemaVersion}
	if _, err := c.Append(context.Background(), []domain.LogEntry{entry}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(endpoint, "/")+"/internal/force_flush", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("force flush returned %s", resp.Status)
	}
	rows, err := c.Recent(context.Background(), 1000)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row["id"] == id {
			return
		}
	}
	t.Fatalf("event %q not returned from VictoriaLogs", id)
}
