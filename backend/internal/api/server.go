package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/controlstore"
	"github.com/freezxp/syslogx/backend/internal/domain"
	"github.com/freezxp/syslogx/backend/internal/storage"
	"github.com/freezxp/syslogx/backend/internal/storage/victorialogs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type ControlChecker interface {
	Check(context.Context, time.Duration) error
}

type HTTPObserver interface {
	Observe(method, route string, status int, duration time.Duration)
}

type Server struct {
	http           *http.Server
	backend        storage.Backend
	query          storage.Querier
	recent         *victorialogs.Client
	control        ControlChecker
	controlTimeout time.Duration
	accepting      *atomic.Bool
	logger         *slog.Logger
	auth           *authManager
	savedMu        sync.RWMutex
	saved          map[string]savedSearch
	savedStore     controlstore.SavedSearchStore
	sources        []config.SourceConfig
}

type Options struct {
	SavedSearches controlstore.SavedSearchStore
	Sources       []config.SourceConfig
}

func New(cfg config.HTTPConfig, backend storage.Backend, control ControlChecker, controlTimeout time.Duration, accepting *atomic.Bool, observer HTTPObserver, logger *slog.Logger, options ...Options) *Server {
	s := &Server{backend: backend, control: control, controlTimeout: controlTimeout, accepting: accepting, logger: logger, auth: newAuthManager(), saved: map[string]savedSearch{}}
	if len(options) > 0 {
		s.savedStore = options[0].SavedSearches
		s.sources = append([]config.SourceConfig(nil), options[0].Sources...)
	}
	if q, ok := backend.(storage.Querier); ok {
		s.query = q
	}
	if v, ok := backend.(*victorialogs.Client); ok {
		s.recent = v
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /api/v1/capabilities", s.capabilities)
	mux.HandleFunc("GET /api/v1/system/logs/recent", s.recentLogs)
	mux.HandleFunc("POST /api/v1/ingest", s.ingestJSON)
	mux.HandleFunc("POST /api/v1/logs/search", s.searchLogs)
	mux.HandleFunc("POST /api/v1/logs/stats", s.logStats)
	mux.HandleFunc("POST /api/v1/fields", s.fieldNames)
	mux.HandleFunc("POST /api/v1/fields/{field}/values", s.fieldValues)
	mux.HandleFunc("POST /api/v1/logs/export", s.exportLogs)
	mux.HandleFunc("GET /api/v1/logs/tail", s.tailLogs)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("GET /api/v1/saved-searches", s.listSaved)
	mux.HandleFunc("POST /api/v1/saved-searches", s.createSaved)
	mux.HandleFunc("DELETE /api/v1/saved-searches/{id}", s.deleteSaved)
	mux.HandleFunc("GET /api/v1/sources", s.listSources)
	s.http = &http.Server{Addr: cfg.Address, Handler: requestLog(logger, observer, s.authMiddleware(mux)), ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout, ReadHeaderTimeout: cfg.ReadHeaderTimeout}
	return s
}

func decodeQuery(r *http.Request) (storage.Query, error) {
	var q storage.Query
	if err := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&q); err != nil {
		return q, err
	}
	q.TenantID = "default"
	return q, nil
}
func (s *Server) requireQuery(w http.ResponseWriter) bool {
	if s.query == nil {
		writeJSON(w, 501, map[string]any{"code": "capability_not_supported"})
		return false
	}
	return true
}
func (s *Server) searchLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	q, e := decodeQuery(r)
	if e != nil {
		writeJSON(w, 400, map[string]any{"code": "invalid_request", "message": e.Error()})
		return
	}
	out, e := s.query.Query(r.Context(), q)
	if e != nil {
		s.queryError(w, e)
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) logStats(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	q, e := decodeQuery(r)
	if e != nil {
		writeJSON(w, 400, map[string]any{"code": "invalid_request"})
		return
	}
	out, e := s.query.Stats(r.Context(), q)
	if e != nil {
		s.queryError(w, e)
		return
	}
	writeJSON(w, 200, out)
}
func (s *Server) fieldNames(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	q, e := decodeQuery(r)
	if e != nil {
		writeJSON(w, 400, map[string]any{"code": "invalid_request"})
		return
	}
	out, e := s.query.FieldNames(r.Context(), q)
	if e != nil {
		s.queryError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Server) fieldValues(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	q, e := decodeQuery(r)
	if e != nil {
		writeJSON(w, 400, map[string]any{"code": "invalid_request"})
		return
	}
	out, e := s.query.FieldValues(r.Context(), q, r.PathValue("field"), 20)
	if e != nil {
		s.queryError(w, e)
		return
	}
	writeJSON(w, 200, map[string]any{"data": out})
}
func (s *Server) exportLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "ndjson"
	}
	q, e := decodeQuery(r)
	if e != nil {
		writeJSON(w, 400, map[string]any{"code": "invalid_request"})
		return
	}
	types := map[string]string{"json": "application/json", "ndjson": "application/x-ndjson", "csv": "text/csv"}
	if types[format] == "" {
		writeJSON(w, 400, map[string]any{"code": "invalid_format"})
		return
	}
	w.Header().Set("Content-Type", types[format])
	w.Header().Set("Content-Disposition", "attachment; filename=logs."+format)
	if e = s.query.Export(r.Context(), q, format, w); e != nil {
		s.logger.Error("export failed", "error", e)
	}
}
func (s *Server) queryError(w http.ResponseWriter, e error) {
	s.logger.Warn("query failed", "error", e)
	writeJSON(w, 400, map[string]any{"code": "query_error", "message": e.Error()})
}

func (s *Server) ingestJSON(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	items := []map[string]any{}
	if strings.Contains(r.Header.Get("Content-Type"), "ndjson") {
		for {
			var m map[string]any
			e := dec.Decode(&m)
			if e == io.EOF {
				break
			}
			if e != nil {
				writeJSON(w, 400, map[string]any{"code": "invalid_ndjson"})
				return
			}
			items = append(items, m)
			if len(items) > 1000 {
				break
			}
		}
	} else {
		var raw any
		if e := dec.Decode(&raw); e != nil {
			writeJSON(w, 400, map[string]any{"code": "invalid_json"})
			return
		}
		switch v := raw.(type) {
		case map[string]any:
			items = append(items, v)
		case []any:
			for _, x := range v {
				m, ok := x.(map[string]any)
				if !ok {
					writeJSON(w, 400, map[string]any{"code": "invalid_batch"})
					return
				}
				items = append(items, m)
			}
		default:
			writeJSON(w, 400, map[string]any{"code": "invalid_json"})
			return
		}
	}
	if len(items) > 1000 {
		writeJSON(w, 413, map[string]any{"code": "batch_too_large"})
		return
	}
	logs := make([]domain.LogEntry, 0, len(items))
	now := time.Now().UTC()
	for i, m := range items {
		msg := fmt.Sprint(m["message"])
		if msg == "<nil>" || msg == "" {
			writeJSON(w, 400, map[string]any{"code": "message_required", "index": i})
			return
		}
		stamp := now
		if v, ok := m["timestamp"].(string); ok && v != "" {
			t, e := time.Parse(time.RFC3339Nano, v)
			if e != nil {
				writeJSON(w, 400, map[string]any{"code": "invalid_timestamp", "index": i})
				return
			}
			stamp = t
		}
		fields := map[string]any{}
		for k, v := range m {
			switch k {
			case "timestamp", "message", "hostname", "level", "severity", "service", "app_name":
			default:
				fields[k] = v
			}
		}
		severity := fmt.Sprint(m["severity"])
		if severity == "<nil>" {
			severity = fmt.Sprint(m["level"])
		}
		app := fmt.Sprint(m["app_name"])
		if app == "<nil>" {
			app = fmt.Sprint(m["service"])
		}
		host := fmt.Sprint(m["hostname"])
		if host == "<nil>" {
			host = ""
		}
		logs = append(logs, domain.LogEntry{ID: fmt.Sprintf("http-%d-%d", now.UnixNano(), i), TenantID: "default", Timestamp: stamp, ReceivedAt: now, Message: msg, Hostname: host, SeverityName: severity, AppName: app, Protocol: "http", Format: "json", SourceID: "http-json", Source: "HTTP JSON", SourceType: "http", Fields: fields, Labels: map[string]string{}, ParseStatus: domain.ParseStatusParsed, SchemaVersion: domain.SchemaVersion})
	}
	out, e := s.backend.Append(r.Context(), logs)
	if e != nil {
		writeJSON(w, 503, map[string]any{"code": "storage_unavailable"})
		return
	}
	writeJSON(w, 202, map[string]any{"accepted": out.Accepted})
}
func (s *Server) tailLogs(w http.ResponseWriter, r *http.Request) {
	if !s.requireQuery(w) {
		return
	}
	text := r.URL.Query().Get("query")
	if len(text) > 4096 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": "query_too_large"})
		return
	}
	f, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"code": "streaming_unsupported"})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprint(w, ": connected\n\n")
	f.Flush()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	seen := make(map[string]struct{}, 4096)
	order := make([]string, 0, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		case now := <-ticker.C:
			_ = controller.SetWriteDeadline(now.Add(5 * time.Second))
			q := storage.Query{TenantID: "default", Start: now.Add(-30 * time.Second), End: now, Limit: 1000, Text: text}
			res, err := s.query.Query(r.Context(), q)
			if err != nil {
				s.logger.Warn("live tail query failed", "error", err)
				fmt.Fprint(w, "event: warning\ndata: {\"code\":\"query_failed\"}\n\n")
			} else {
				if res.Truncated {
					fmt.Fprint(w, "event: gap\ndata: {\"code\":\"tail_window_truncated\"}\n\n")
				}
				for i := len(res.Data) - 1; i >= 0; i-- {
					row := res.Data[i]
					key := tailKey(row)
					if _, exists := seen[key]; exists {
						continue
					}
					seen[key] = struct{}{}
					order = append(order, key)
					if len(order) > 4096 {
						delete(seen, order[0])
						order = order[1:]
					}
					b, err := json.Marshal(row)
					if err != nil {
						continue
					}
					if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
						return
					}
				}
			}
			fmt.Fprint(w, ": heartbeat\n\n")
			f.Flush()
		}
	}
}

func tailKey(row map[string]any) string {
	if id, ok := row["id"].(string); ok && id != "" {
		return id
	}
	return fmt.Sprint(row["_stream_id"]) + "|" + fmt.Sprint(row["_time"]) + "|" + fmt.Sprint(row["_msg"])
}
func (s *Server) ListenAndServe() error              { return s.http.ListenAndServe() }
func (s *Server) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	result := map[string]any{"status": "ready", "checks": map[string]any{}}
	checks := result["checks"].(map[string]any)
	status := http.StatusOK
	if !s.accepting.Load() {
		checks["ingestion"] = "draining"
		status = http.StatusServiceUnavailable
	} else {
		checks["ingestion"] = "accepting"
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	h := s.backend.Check(ctx)
	checks["storage"] = map[string]bool{"healthy": h.Healthy}
	if !h.Healthy {
		status = http.StatusServiceUnavailable
	}
	if s.control != nil {
		if err := s.control.Check(r.Context(), s.controlTimeout); err != nil {
			checks["control_store"] = "unavailable"
			status = http.StatusServiceUnavailable
		} else {
			checks["control_store"] = "ok"
		}
	}
	if status != http.StatusOK {
		result["status"] = "not_ready"
	}
	writeJSON(w, status, result)
}

func (s *Server) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.backend.Capabilities())
}

func (s *Server) recentLogs(w http.ResponseWriter, r *http.Request) {
	if s.recent == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"code": "capability_not_supported"})
		return
	}
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_limit"})
			return
		}
		if n < 1 || n > 1000 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": "invalid_limit"})
			return
		}
		limit = n
	}
	rows, err := s.recent.Recent(r.Context(), limit)
	if err != nil {
		s.logger.Error("recent log query failed", "error", err)
		writeJSON(w, http.StatusBadGateway, map[string]any{"code": "storage_error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": rows})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
func (w *statusWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(p)
}
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func requestLog(logger *slog.Logger, observer HTTPObserver, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)
		if observer != nil {
			observer.Observe(r.Method, r.Pattern, recorder.status, duration)
		}
		logger.Info("http request", "method", r.Method, "route", r.Pattern, "status", recorder.status, "duration_ms", duration.Milliseconds())
	})
}
