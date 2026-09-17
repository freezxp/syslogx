package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type options struct {
	target, protocol, format, path string
	port, rate, count              int
	seed                           int64
}

func main() {
	var o options
	flag.StringVar(&o.target, "target", "127.0.0.1", "destination host")
	flag.StringVar(&o.protocol, "protocol", "udp", "udp, tcp, or http")
	flag.StringVar(&o.format, "format", "rfc5424", "rfc5424, rfc3164, or json")
	flag.StringVar(&o.path, "path", "/api/v1/ingest", "HTTP ingestion path")
	flag.IntVar(&o.port, "port", 514, "destination port")
	flag.IntVar(&o.rate, "rate", 1000, "target logs per second")
	flag.IntVar(&o.count, "count", 1000, "total logs to send")
	flag.Int64Var(&o.seed, "seed", 1, "random seed for reproducibility")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	start := time.Now()
	sent, err := run(ctx, o)
	elapsed := time.Since(start)
	fmt.Fprintf(os.Stderr, "sent=%d requested=%d elapsed=%s measured_rate=%.1f/s\n", sent, o.count, elapsed.Round(time.Millisecond), float64(sent)/elapsed.Seconds())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (o options) validate() error {
	if o.rate < 1 || o.rate > 1_000_000 || o.count < 1 || o.port < 1 || o.port > 65535 {
		return errors.New("rate, count, or port out of range")
	}
	if o.protocol != "udp" && o.protocol != "tcp" && o.protocol != "http" {
		return errors.New("protocol must be udp, tcp, or http")
	}
	if o.format != "rfc3164" && o.format != "rfc5424" && o.format != "json" {
		return errors.New("format must be rfc3164, rfc5424, or json")
	}
	if (o.protocol == "http") != (o.format == "json") {
		return errors.New("HTTP requires JSON; UDP/TCP require RFC3164 or RFC5424")
	}
	if strings.TrimSpace(o.target) == "" || !strings.HasPrefix(o.path, "/") {
		return errors.New("invalid target or HTTP path")
	}
	return nil
}

func run(ctx context.Context, o options) (int, error) {
	if err := o.validate(); err != nil {
		return 0, err
	}
	rng := rand.New(rand.NewSource(o.seed))
	address := net.JoinHostPort(o.target, strconv.Itoa(o.port))
	var conn net.Conn
	var writer *bufio.Writer
	if o.protocol != "http" {
		var err error
		conn, err = net.DialTimeout(o.protocol, address, 5*time.Second)
		if err != nil {
			return 0, err
		}
		defer conn.Close()
		if o.protocol == "tcp" {
			writer = bufio.NewWriterSize(conn, 64<<10)
		}
	}
	client := &http.Client{Timeout: 20 * time.Second}
	endpoint := "http://" + address + o.path
	sent := 0
	start := time.Now()
	for sent < o.count {
		targetByNow := int(time.Since(start).Seconds()*float64(o.rate)) + max(1, o.rate/10)
		if targetByNow > o.count {
			targetByNow = o.count
		}
		if targetByNow <= sent {
			select {
			case <-ctx.Done():
				return sent, ctx.Err()
			case <-time.After(10 * time.Millisecond):
			}
			continue
		}
		for sent < targetByNow {
			if err := ctx.Err(); err != nil {
				return sent, err
			}
			if o.protocol == "http" {
				batchSize := min(1000, targetByNow-sent)
				batch := make([]jsonLog, 0, batchSize)
				for i := 0; i < batchSize; i++ {
					batch = append(batch, makeJSON(rng, sent+i))
				}
				body, err := json.Marshal(batch)
				if err != nil {
					return sent, err
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
				if err != nil {
					return sent, err
				}
				req.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(req)
				if err != nil {
					return sent, err
				}
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				if resp.StatusCode != http.StatusAccepted {
					return sent, fmt.Errorf("HTTP ingestion returned %s", resp.Status)
				}
				sent += batchSize
				continue
			}
			message := syslogLine(rng, o.format, sent)
			if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return sent, err
			}
			var err error
			if writer != nil {
				_, err = writer.WriteString(message + "\n")
			} else {
				_, err = io.WriteString(conn, message)
			}
			if err != nil {
				return sent, err
			}
			sent++
		}
		if writer != nil {
			if err := writer.Flush(); err != nil {
				return sent, err
			}
		}
	}
	return sent, nil
}

type jsonLog struct {
	Timestamp  string `json:"timestamp"`
	Hostname   string `json:"hostname"`
	Level      string `json:"level"`
	Service    string `json:"service"`
	Message    string `json:"message"`
	SourceIP   string `json:"source_ip"`
	Vendor     string `json:"vendor"`
	DeviceType string `json:"device_type"`
	EventID    int    `json:"event_id"`
}

var hosts = []string{"fw01", "fw02", "web01", "db01"}
var apps = []string{"vpn", "firewall", "nginx", "sshd"}
var levels = []string{"debug", "info", "notice", "warning", "error", "critical", "alert", "emergency"}

func makeJSON(rng *rand.Rand, id int) jsonLog {
	host := hosts[rng.Intn(len(hosts))]
	app := apps[rng.Intn(len(apps))]
	return jsonLog{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Hostname:  host, Level: levels[rng.Intn(len(levels))], Service: app,
		Message:  fmt.Sprintf("synthetic %s event %d", app, id),
		SourceIP: fmt.Sprintf("10.10.%d.%d", rng.Intn(256), 1+rng.Intn(254)),
		Vendor:   "syslogx", DeviceType: "synthetic", EventID: id,
	}
}

func syslogLine(rng *rand.Rand, format string, id int) string {
	host := hosts[rng.Intn(len(hosts))]
	app := apps[rng.Intn(len(apps))]
	severity := rng.Intn(8)
	facility := 16 + rng.Intn(8)
	priority := facility*8 + severity
	msg := fmt.Sprintf("synthetic %s event %d", app, id)
	if format == "rfc3164" {
		return fmt.Sprintf("<%d>%s %s %s[%d]: %s", priority, time.Now().Format("Jan _2 15:04:05"), host, app, 1000+id%9000, msg)
	}
	ip := fmt.Sprintf("10.10.%d.%d", rng.Intn(256), 1+rng.Intn(254))
	return fmt.Sprintf("<%d>1 %s %s %s %d synthetic [syslogx@32473 source_ip=%q event_id=%q] %s", priority, time.Now().UTC().Format(time.RFC3339Nano), host, app, 1000+id%9000, ip, strconv.Itoa(id), msg)
}
