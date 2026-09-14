package storage

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
)

var ErrUnavailable = errors.New("storage unavailable")

type AppendResult struct {
	Accepted int
	Bytes    int
}

type Health struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type Capabilities struct {
	Backend        string `json:"backend"`
	FullText       bool   `json:"full_text"`
	FieldDiscovery bool   `json:"field_discovery"`
	Facets         bool   `json:"facets"`
	Statistics     bool   `json:"statistics"`
	LiveTail       bool   `json:"live_tail"`
	NativeDialect  string `json:"native_dialect,omitempty"`
}

type Appender interface {
	Append(context.Context, []domain.LogEntry) (AppendResult, error)
}

type Backend interface {
	Appender
	Check(context.Context) Health
	Capabilities() Capabilities
}

type Filter struct {
	Field string `json:"field"`
	Op    string `json:"operator"`
	Value any    `json:"value,omitempty"`
}
type Query struct {
	TenantID string    `json:"-"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Text     string    `json:"query,omitempty"`
	Native   string    `json:"native_query,omitempty"`
	Filters  []Filter  `json:"filters,omitempty"`
	Limit    int       `json:"limit"`
	Cursor   string    `json:"cursor,omitempty"`
}
type QueryResult struct {
	Data       []map[string]any `json:"data"`
	NextCursor string           `json:"next_cursor,omitempty"`
	Truncated  bool             `json:"truncated"`
}
type Bucket struct {
	Timestamp time.Time `json:"timestamp"`
	Count     int64     `json:"count"`
}
type ValueCount struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}
type StatsResult struct {
	Total           int64        `json:"total"`
	Volume          []Bucket     `json:"volume"`
	Severities      []ValueCount `json:"severities"`
	TopHosts        []ValueCount `json:"top_hosts"`
	TopApplications []ValueCount `json:"top_applications"`
}
type FieldInfo struct {
	Name  string `json:"name"`
	Count int64  `json:"count,omitempty"`
}
type Querier interface {
	Query(context.Context, Query) (QueryResult, error)
	Stats(context.Context, Query) (StatsResult, error)
	FieldNames(context.Context, Query) ([]FieldInfo, error)
	FieldValues(context.Context, Query, string, int) ([]ValueCount, error)
	Export(context.Context, Query, string, io.Writer) error
}
