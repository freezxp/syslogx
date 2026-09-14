package parser

import (
	"context"
	"testing"
	"time"
)

func TestRFC5424NilTimestampUsesReceiveTime(t *testing.T) {
	received := time.Date(2026, 9, 14, 14, 30, 0, 0, time.UTC)
	e, err := (RFC5424{}).Parse(context.Background(), []byte(`<34>1 - host app proc msg - message`), received, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if !e.Timestamp.Equal(received) || !e.TimestampInferred {
		t.Fatalf("timestamp=%s inferred=%v", e.Timestamp, e.TimestampInferred)
	}
}
