package storage

import (
	"context"
	"errors"

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
