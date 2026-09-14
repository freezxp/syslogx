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
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/freezxp/syslogx/backend/internal/domain"
	querybuilder "github.com/freezxp/syslogx/backend/internal/query"
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

func (c *Client) Query(ctx context.Context, q storage.Query) (storage.QueryResult, error) {
	base, err := querybuilder.LogsQL(q)
	if err != nil {
		return storage.QueryResult{}, err
	}
	rows, err := c.runQuery(ctx, base+" | sort by (_time) desc | limit "+strconv.Itoa(q.Limit+1))
	if err != nil {
		return storage.QueryResult{}, err
	}
	result := storage.QueryResult{Data: rows}
	if len(rows) > q.Limit {
		result.Data = rows[:q.Limit]
		result.Truncated = true
	}
	if result.Truncated && len(result.Data) > 0 {
		if t, ok := result.Data[len(result.Data)-1]["_time"].(string); ok {
			result.NextCursor = querybuilder.Cursor(t)
		}
	}
	return result, nil
}

func (c *Client) runQuery(ctx context.Context, logsQL string) ([]map[string]any, error) {
	u := *c.endpoint
	u.Path = strings.TrimRight(u.Path, "/") + "/select/logsql/query"
	form := url.Values{"query": {logsQL}}
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
	rows := []map[string]any{}
	s := bufio.NewScanner(resp.Body)
	s.Buffer(make([]byte, 64<<10), 4<<20)
	for s.Scan() {
		var row map[string]any
		if err := json.Unmarshal(s.Bytes(), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, s.Err()
}

func (c *Client) Stats(ctx context.Context, q storage.Query) (storage.StatsResult, error) {
	copyQ := q
	copyQ.Limit = 1000
	copyQ.Cursor = ""
	res, err := c.Query(ctx, copyQ)
	if err != nil {
		return storage.StatsResult{}, err
	}
	out := storage.StatsResult{Total: int64(len(res.Data))}
	sev := map[string]int64{}
	hosts := map[string]int64{}
	apps := map[string]int64{}
	buckets := map[time.Time]int64{}
	for _, r := range res.Data {
		count(sev, r["severity_name"])
		count(hosts, r["hostname"])
		count(apps, r["app_name"])
		if raw, ok := r["_time"].(string); ok {
			if t, e := time.Parse(time.RFC3339Nano, raw); e == nil {
				t = t.Truncate(time.Minute)
				buckets[t]++
			}
		}
	}
	out.Severities = sortedCounts(sev, 20)
	out.TopHosts = sortedCounts(hosts, 10)
	out.TopApplications = sortedCounts(apps, 10)
	for t, n := range buckets {
		out.Volume = append(out.Volume, storage.Bucket{Timestamp: t, Count: n})
	}
	sort.Slice(out.Volume, func(i, j int) bool { return out.Volume[i].Timestamp.Before(out.Volume[j].Timestamp) })
	return out, nil
}
func count(m map[string]int64, v any) {
	s := fmt.Sprint(v)
	if s != "" && s != "<nil>" {
		m[s]++
	}
}
func sortedCounts(m map[string]int64, limit int) []storage.ValueCount {
	a := make([]storage.ValueCount, 0, len(m))
	for k, v := range m {
		a = append(a, storage.ValueCount{Value: k, Count: v})
	}
	sort.Slice(a, func(i, j int) bool { return a[i].Count > a[j].Count })
	if len(a) > limit {
		a = a[:limit]
	}
	return a
}
func (c *Client) FieldNames(ctx context.Context, q storage.Query) ([]storage.FieldInfo, error) {
	copyQ := q
	copyQ.Limit = 500
	res, err := c.Query(ctx, copyQ)
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	for _, r := range res.Data {
		for k := range r {
			m[k]++
		}
	}
	out := make([]storage.FieldInfo, 0, len(m))
	for k, n := range m {
		out = append(out, storage.FieldInfo{Name: k, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}
func (c *Client) FieldValues(ctx context.Context, q storage.Query, field string, limit int) ([]storage.ValueCount, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	copyQ := q
	copyQ.Limit = 1000
	res, err := c.Query(ctx, copyQ)
	if err != nil {
		return nil, err
	}
	m := map[string]int64{}
	for _, r := range res.Data {
		count(m, r[field])
	}
	return sortedCounts(m, limit), nil
}
func (c *Client) Export(ctx context.Context, q storage.Query, format string, w io.Writer) error {
	q.Limit = 1000
	res, err := c.Query(ctx, q)
	if err != nil {
		return err
	}
	switch format {
	case "json":
		return json.NewEncoder(w).Encode(res.Data)
	case "ndjson":
		e := json.NewEncoder(w)
		for _, r := range res.Data {
			if err := e.Encode(r); err != nil {
				return err
			}
		}
		return nil
	case "csv":
		keys := []string{"_time", "hostname", "severity_name", "app_name", "message"}
		fmt.Fprintln(w, strings.Join(keys, ","))
		for _, r := range res.Data {
			vals := make([]string, len(keys))
			for i, k := range keys {
				vals[i] = strconv.Quote(fmt.Sprint(r[k]))
			}
			fmt.Fprintln(w, strings.Join(vals, ","))
		}
		return nil
	default:
		return fmt.Errorf("unsupported export format")
	}
}
