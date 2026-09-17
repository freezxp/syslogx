package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/freezxp/syslogx/backend/internal/parser"
)

func TestGeneratedSyslogParses(t *testing.T) {
	for _, format := range []string{"rfc3164", "rfc5424"} {
		t.Run(format, func(t *testing.T) {
			line := syslogLine(rand.New(rand.NewSource(1)), format, 42)
			event, err := parser.NewRegistry().Parse(context.Background(), "auto", []byte(line), time.Now(), time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			if event.Format != format || event.Hostname == "" || !strings.Contains(event.Message, "event 42") {
				t.Fatalf("event=%+v", event)
			}
			if format == "rfc5424" && event.Fields["syslog.sd.syslogx@32473.event_id"] != "42" {
				t.Fatalf("fields=%+v", event.Fields)
			}
		})
	}
}

func TestHTTPGeneratorBatchesJSON(t *testing.T) {
	received := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ingest" {
			t.Errorf("path=%s", r.URL.Path)
		}
		var batch []jsonLog
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			t.Error(err)
		}
		received += len(batch)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	number, _ := strconv.Atoi(port)
	sent, err := run(context.Background(), options{target: host, port: number, protocol: "http", format: "json", path: "/api/v1/ingest", rate: 10000, count: 1200, seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if sent != 1200 || received != sent {
		t.Fatalf("sent=%d received=%d", sent, received)
	}
}

func TestInvalidProtocolFormatCombination(t *testing.T) {
	o := options{target: "127.0.0.1", port: 514, rate: 1000, count: 1, protocol: "udp", format: "json", path: "/api/v1/ingest"}
	if err := o.validate(); err == nil {
		t.Fatal("JSON over UDP was accepted")
	}
}
