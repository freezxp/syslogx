package domain

import "time"

const SchemaVersion uint16 = 1

type ParseStatus string

const (
	ParseStatusParsed  ParseStatus = "parsed"
	ParseStatusPartial ParseStatus = "partial"
	ParseStatusUnknown ParseStatus = "unknown"
	ParseStatusInvalid ParseStatus = "invalid"
)

type LogEntry struct {
	ID                string            `json:"id"`
	TenantID          string            `json:"tenant_id"`
	Timestamp         time.Time         `json:"timestamp"`
	ReceivedAt        time.Time         `json:"received_at"`
	Message           string            `json:"message"`
	RawMessage        string            `json:"raw_message,omitempty"`
	Hostname          string            `json:"hostname,omitempty"`
	SourceIP          string            `json:"source_ip,omitempty"`
	SourcePort        uint16            `json:"source_port,omitempty"`
	Facility          *uint8            `json:"facility,omitempty"`
	FacilityName      string            `json:"facility_name,omitempty"`
	Severity          *uint8            `json:"severity,omitempty"`
	SeverityName      string            `json:"severity_name,omitempty"`
	Priority          *uint8            `json:"priority,omitempty"`
	Protocol          string            `json:"protocol"`
	Format            string            `json:"format"`
	AppName           string            `json:"app_name,omitempty"`
	ProcessID         string            `json:"process_id,omitempty"`
	MessageID         string            `json:"message_id,omitempty"`
	SourceID          string            `json:"source_id"`
	Source            string            `json:"source,omitempty"`
	SourceType        string            `json:"source_type"`
	Fields            map[string]any    `json:"fields"`
	Labels            map[string]string `json:"labels"`
	ParseStatus       ParseStatus       `json:"parse_status"`
	SchemaVersion     uint16            `json:"schema_version"`
	TimestampInferred bool              `json:"timestamp_inferred,omitempty"`
}

type Frame struct {
	Payload    []byte
	ReceivedAt time.Time
	TenantID   string
	SourceID   string
	SourceName string
	Protocol   string
	SourceIP   string
	SourcePort uint16
}
