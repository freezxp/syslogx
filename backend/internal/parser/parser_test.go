package parser

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRFC5424(t *testing.T) {
	input := `<165>1 2026-09-14T14:30:00.123Z fw01 vpn 8710 ID47 [exampleSDID@32473 iut="3" eventSource="Application" note="a space and \] bracket"] VPN disconnected`
	e, err := (RFC5424{}).Parse(context.Background(), []byte(input), time.Time{}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if e.Hostname != "fw01" || e.AppName != "vpn" || e.ProcessID != "8710" || e.MessageID != "ID47" || e.Message != "VPN disconnected" {
		t.Fatalf("unexpected event: %#v", e)
	}
	if e.Priority == nil || *e.Priority != 165 || e.Facility == nil || *e.Facility != 20 || e.Severity == nil || *e.Severity != 5 {
		t.Fatalf("unexpected priority: %#v", e)
	}
	if got := e.Fields["syslog.sd.exampleSDID@32473.note"]; got != "a space and ] bracket" {
		t.Fatalf("structured data note = %#v", got)
	}
}

func TestRFC5424NilValues(t *testing.T) {
	e, err := (RFC5424{}).Parse(context.Background(), []byte(`<34>1 2026-09-14T14:30:00Z - - - - - message`), time.Time{}, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if e.Hostname != "" || e.AppName != "" || e.ProcessID != "" || e.MessageID != "" || e.Message != "message" {
		t.Fatalf("unexpected event: %#v", e)
	}
}

func TestRFC3164(t *testing.T) {
	received := time.Date(2026, time.January, 1, 0, 0, 10, 0, time.UTC)
	e, err := (RFC3164{}).Parse(context.Background(), []byte(`<34>Dec 31 23:59:59 host sshd[123]: failed login`), received, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if e.Timestamp.Year() != 2025 || e.Hostname != "host" || e.AppName != "sshd" || e.ProcessID != "123" || e.Message != "failed login" {
		t.Fatalf("unexpected event: %#v", e)
	}
}

func TestRegistryUnknownPreservesMessage(t *testing.T) {
	r := NewRegistry()
	received := time.Now().UTC()
	e, err := r.Parse(context.Background(), "auto", []byte("plain event"), received, time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	if e.Format != "unknown" || e.Message != "plain event" || !e.Timestamp.Equal(received) {
		t.Fatalf("unexpected event: %#v", e)
	}
}

func TestInvalidPriority(t *testing.T) {
	_, err := (RFC5424{}).Parse(context.Background(), []byte(`<999>1 2026-09-14T14:30:00Z h a p m - x`), time.Time{}, time.UTC)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected ErrInvalid, got %v", err)
	}
}

func FuzzRFC5424NeverPanics(f *testing.F) {
	f.Add([]byte(`<34>1 2026-09-14T14:30:00Z h a p m - message`))
	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = (RFC5424{}).Parse(context.Background(), input, time.Now(), time.UTC)
	})
}

func FuzzRFC3164NeverPanics(f *testing.F) {
	f.Add([]byte(`<34>Sep 14 14:30:00 h app[1]: message`))
	f.Fuzz(func(t *testing.T, input []byte) {
		_, _ = (RFC3164{}).Parse(context.Background(), input, time.Now(), time.UTC)
	})
}
