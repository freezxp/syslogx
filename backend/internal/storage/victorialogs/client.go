package victorialogs

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/storage"
)

type Client struct {
	endpoint     *url.URL
	httpClient   *http.Client
	streamFields string
}

func New(endpoint string, timeout time.Duration, streamFields []string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid VictoriaLogs endpoint")
	}
	return &Client{endpoint: u, httpClient: &http.Client{Timeout: timeout}, streamFields: strings.Join(streamFields, ",")}, nil
}

func (c *Client) Capabilities() storage.Capabilities {
	return storage.Capabilities{Backend: "victorialogs", FullText: true, FieldDiscovery: true, Facets: true, Statistics: true, LiveTail: true, NativeDialect: "logsql"}
}

func (c *Client) Check(ctx context.Context) storage.Health {
	u := *c.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return storage.Health{Message: err.Error()}
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return storage.Health{Message: err.Error()}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return storage.Health{Message: resp.Status}
	}
	return storage.Health{Healthy: true}
}

func (c *Client) Append(ctx context.Context, logs []domain.LogEntry) (storage.AppendResult, error) {
	if len(logs) == 0 {
		return storage.AppendResult{}, nil
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for i := range logs {
		if err := enc.Encode(toDirectRow(logs[i])); err != nil {
			return storage.AppendResult{}, fmt.Errorf("encode log %d: %w", i, err)
		}
	}
	u := *c.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/insert/jsonline"
	encodedBytes := body.Len()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &body)
	if err != nil {
		return storage.AppendResult{}, err
	}
	req.Header.Set("Content-Type", "application/stream+json")
	req.Header.Set("VL-Msg-Field", "message")
	req.Header.Set("VL-Time-Field", "timestamp")
	if c.streamFields != "" {
		req.Header.Set("VL-Stream-Fields", c.streamFields)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return storage.AppendResult{}, fmt.Errorf("%w: %v", storage.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return storage.AppendResult{}, fmt.Errorf("%w: VictoriaLogs returned %s: %s", storage.ErrUnavailable, resp.Status, strings.TrimSpace(string(msg)))
	}
	return storage.AppendResult{Accepted: len(logs), Bytes: encodedBytes}, nil
}

func (c *Client) Recent(ctx context.Context, limit int) ([]map[string]any, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	u := *c.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/select/logsql/query"
	form := url.Values{"query": {"_time:15m | sort by (_time) desc | limit " + strconv.Itoa(limit)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", storage.ErrUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("VictoriaLogs query returned %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	rows := make([]map[string]any, 0, limit)
	s := bufio.NewScanner(resp.Body)
	s.Buffer(make([]byte, 64<<10), 2<<20)
	for s.Scan() {
		var row map[string]any
		if err := json.Unmarshal(s.Bytes(), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
		if len(rows) >= limit {
			break
		}
	}
	return rows, s.Err()
}
