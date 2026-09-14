package normalization

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/parser"
)

var facilityNames = [...]string{"kernel", "user", "mail", "daemon", "auth", "syslog", "lpr", "news", "uucp", "clock", "authpriv", "ftp", "ntp", "audit", "alert", "clock2", "local0", "local1", "local2", "local3", "local4", "local5", "local6", "local7"}
var severityNames = [...]string{"emergency", "alert", "critical", "error", "warning", "notice", "informational", "debug"}

func Normalize(frame domain.Frame, event parser.Event) (domain.LogEntry, error) {
	id, err := uuidV7(frame.ReceivedAt)
	if err != nil {
		return domain.LogEntry{}, fmt.Errorf("generate event id: %w", err)
	}
	entry := domain.LogEntry{ID: id, TenantID: frame.TenantID, Timestamp: event.Timestamp.UTC(), ReceivedAt: frame.ReceivedAt.UTC(), Message: strings.ToValidUTF8(event.Message, "�"), RawMessage: strings.ToValidUTF8(string(frame.Payload), "�"), Hostname: event.Hostname, SourceIP: frame.SourceIP, SourcePort: frame.SourcePort, Facility: event.Facility, Severity: event.Severity, Priority: event.Priority, Protocol: frame.Protocol, Format: event.Format, AppName: event.AppName, ProcessID: event.ProcessID, MessageID: event.MessageID, SourceID: frame.SourceID, Source: frame.SourceName, SourceType: "syslog", Fields: event.Fields, Labels: map[string]string{}, ParseStatus: domain.ParseStatusParsed, SchemaVersion: domain.SchemaVersion, TimestampInferred: event.TimestampInferred}
	if entry.TenantID == "" {
		entry.TenantID = "default"
	}
	if entry.Fields == nil {
		entry.Fields = map[string]any{}
	}
	if entry.Format == "unknown" {
		entry.ParseStatus = domain.ParseStatusUnknown
	}
	if entry.Facility != nil && int(*entry.Facility) < len(facilityNames) {
		entry.FacilityName = facilityNames[*entry.Facility]
	}
	if entry.Severity != nil && int(*entry.Severity) < len(severityNames) {
		entry.SeverityName = severityNames[*entry.Severity]
	}
	return entry, nil
}

func uuidV7(now time.Time) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	ms := uint64(now.UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	b[6] = (b[6] & 0x0f) | 0x70
	b[8] = (b[8] & 0x3f) | 0x80
	buf := make([]byte, 36)
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf), nil
}
