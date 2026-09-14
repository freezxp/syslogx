package victorialogs

import (
	"strconv"

	"github.com/freezxp/syslogx/backend/internal/domain"
)

// toDirectRow deliberately avoids a marshal/unmarshal conversion per event. This
// path is exercised for every ingested record and must remain allocation-conscious.
func toDirectRow(e domain.LogEntry) map[string]any {
	row := make(map[string]any, 24+len(e.Fields)+len(e.Labels))
	row["id"] = e.ID
	row["tenant_id"] = e.TenantID
	row["timestamp"] = e.Timestamp.Format(timestampFormat)
	row["received_at"] = e.ReceivedAt.Format(timestampFormat)
	row["message"] = e.Message
	row["protocol"] = e.Protocol
	row["format"] = e.Format
	row["source_id"] = e.SourceID
	row["source_type"] = e.SourceType
	row["parse_status"] = string(e.ParseStatus)
	row["schema_version"] = strconv.FormatUint(uint64(e.SchemaVersion), 10)
	optionalString(row, "raw_message", e.RawMessage)
	optionalString(row, "hostname", e.Hostname)
	optionalString(row, "source_ip", e.SourceIP)
	optionalString(row, "facility_name", e.FacilityName)
	optionalString(row, "severity_name", e.SeverityName)
	optionalString(row, "app_name", e.AppName)
	optionalString(row, "process_id", e.ProcessID)
	optionalString(row, "message_id", e.MessageID)
	optionalString(row, "source", e.Source)
	if e.SourcePort != 0 {
		row["source_port"] = strconv.FormatUint(uint64(e.SourcePort), 10)
	}
	if e.Facility != nil {
		row["facility"] = strconv.FormatUint(uint64(*e.Facility), 10)
	}
	if e.Severity != nil {
		row["severity"] = strconv.FormatUint(uint64(*e.Severity), 10)
	}
	if e.Priority != nil {
		row["priority"] = strconv.FormatUint(uint64(*e.Priority), 10)
	}
	if e.TimestampInferred {
		row["timestamp_inferred"] = "true"
	}
	for k, v := range e.Fields {
		if _, reserved := row[k]; reserved {
			row["event."+k] = v
		} else {
			row[k] = v
		}
	}
	for k, v := range e.Labels {
		row["label."+k] = v
	}
	return row
}

const timestampFormat = "2006-01-02T15:04:05.999999999Z07:00"

func optionalString(row map[string]any, key, value string) {
	if value != "" {
		row[key] = value
	}
}
