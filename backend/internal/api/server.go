package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/freezxp/syslogx/backend/internal/config"
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
	recent         *victorialogs.Client
	control        ControlChecker
	controlTimeout time.Duration
	accepting      *atomic.Bool
	logger         *slog.Logger
}

func New(cfg config.HTTPConfig, backend storage.Backend, control ControlChecker, controlTimeout time.Duration, accepting *atomic.Bool, observer HTTPObserver, logger *slog.Logger) *Server {
	s := &Server{backend: backend, control: control, controlTimeout: controlTimeout, accepting: accepting, logger: logger}
	if v, ok := backend.(*victorialogs.Client); ok {
		s.recent = v
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /ready", s.ready)
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /api/v1/capabilities", s.capabilities)
	mux.HandleFunc("GET /api/v1/system/logs/recent", s.recentLogs)
	s.http = &http.Server{Addr: cfg.Address, Handler: requestLog(logger, observer, mux), ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout, ReadHeaderTimeout: cfg.ReadHeaderTimeout}
	return s
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
