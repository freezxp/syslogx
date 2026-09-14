package normalization

import (
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/parser"
)

func TestNormalize(t *testing.T) {
	p, f, s := uint8(164), uint8(20), uint8(4)
	now := time.Now().UTC()
	e, err := Normalize(domain.Frame{Payload: []byte("raw"), ReceivedAt: now, TenantID: "t1", SourceID: "s1", Protocol: "syslog_udp", SourceIP: "127.0.0.1", SourcePort: 514}, parser.Event{Timestamp: now, Message: "message", Format: "rfc5424", Priority: &p, Facility: &f, Severity: &s, Fields: map[string]any{"vendor": "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if e.ID == "" || e.TenantID != "t1" || e.FacilityName != "local4" || e.SeverityName != "warning" || e.SchemaVersion != 1 || e.Fields["vendor"] != "x" {
		t.Fatalf("unexpected normalized event: %#v", e)
	}
}
