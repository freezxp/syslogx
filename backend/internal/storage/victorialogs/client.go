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
	q.Cursor = ""
	base, err := querybuilder.LogsQL(q)
	if err != nil {
		return storage.StatsResult{}, err
	}
	totalRows, err := c.runQuery(ctx, base+" | stats count() as total")
	if err != nil {
		return storage.StatsResult{}, err
	}
	out := storage.StatsResult{}
	if len(totalRows) > 0 {
		out.Total, err = number(totalRows[0]["total"])
		if err != nil {
			return storage.StatsResult{}, err
		}
	}
	step := "1m"
	span := q.End.Sub(q.Start)
	switch {
	case span > 7*24*time.Hour:
		step = "6h"
	case span > 2*24*time.Hour:
		step = "1h"
	case span > 6*time.Hour:
		step = "5m"
	}
	volumeRows, err := c.runQuery(ctx, base+" | stats by (_time:"+step+") count() as hits | sort by (_time) asc | limit 500")
	if err != nil {
		return storage.StatsResult{}, err
	}
	for _, row := range volumeRows {
		t, err := time.Parse(time.RFC3339Nano, fmt.Sprint(row["_time"]))
		if err != nil {
			return storage.StatsResult{}, fmt.Errorf("invalid time bucket: %w", err)
		}
		hits, err := number(row["hits"])
		if err != nil {
			return storage.StatsResult{}, err
		}
		out.Volume = append(out.Volume, storage.Bucket{Timestamp: t, Count: hits})
	}
	out.Severities, err = c.groupedCounts(ctx, base, "severity_name", 20)
	if err != nil {
		return storage.StatsResult{}, err
	}
	out.TopHosts, err = c.groupedCounts(ctx, base, "hostname", 10)
	if err != nil {
		return storage.StatsResult{}, err
	}
	out.TopApplications, err = c.groupedCounts(ctx, base, "app_name", 10)
	return out, err
}

func (c *Client) groupedCounts(ctx context.Context, base, field string, limit int) ([]storage.ValueCount, error) {
	rows, err := c.runQuery(ctx, base+" | stats by ("+field+") count() as hits | sort by (hits) desc | limit "+strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}
	out := make([]storage.ValueCount, 0, len(rows))
	for _, row := range rows {
		value := fmt.Sprint(row[field])
		if value == "" || value == "<nil>" {
			continue
		}
		hits, err := number(row["hits"])
		if err != nil {
			return nil, err
		}
		out = append(out, storage.ValueCount{Value: value, Count: hits})
	}
	return out, nil
}

func number(v any) (int64, error) {
	n, err := strconv.ParseInt(fmt.Sprint(v), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid VictoriaLogs aggregate %q: %w", v, err)
	}
	return n, nil
}

func (c *Client) FieldNames(ctx context.Context, q storage.Query) ([]storage.FieldInfo, error) {
	q.Cursor = ""
	base, err := querybuilder.LogsQL(q)
	if err != nil {
		return nil, err
	}
	rows, err := c.runQuery(ctx, base+" | field_names | sort by (hits) desc | limit 500")
	if err != nil {
		return nil, err
	}
	out := make([]storage.FieldInfo, 0, len(rows))
	for _, row := range rows {
		hits, err := number(row["hits"])
		if err != nil {
			return nil, err
		}
		out = append(out, storage.FieldInfo{Name: fmt.Sprint(row["name"]), Count: hits})
	}
	return out, nil
}

func (c *Client) FieldValues(ctx context.Context, q storage.Query, field string, limit int) ([]storage.ValueCount, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("limit must be between 1 and 100")
	}
	if !querybuilder.ValidField(field) {
		return nil, fmt.Errorf("invalid field name")
	}
	q.Cursor = ""
	base, err := querybuilder.LogsQL(q)
	if err != nil {
		return nil, err
	}
	rows, err := c.runQuery(ctx, base+" | field_values "+field+" | sort by (hits) desc | limit "+strconv.Itoa(limit))
	if err != nil {
		return nil, err
	}
	out := make([]storage.ValueCount, 0, len(rows))
	for _, row := range rows {
		hits, err := number(row["hits"])
		if err != nil {
			return nil, err
		}
		out = append(out, storage.ValueCount{Value: fmt.Sprint(row[field]), Count: hits})
	}
	return out, nil
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
