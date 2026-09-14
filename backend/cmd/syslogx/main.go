package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"
	_ "time/tzdata"

	"github.com/freezxp/syslogx/backend/internal/api"
	"github.com/freezxp/syslogx/backend/internal/config"
	"github.com/freezxp/syslogx/backend/internal/controlstore/postgres"
	"github.com/freezxp/syslogx/backend/internal/ingestion"
	"github.com/freezxp/syslogx/backend/internal/metrics"
	"github.com/freezxp/syslogx/backend/internal/storage/victorialogs"
	transport "github.com/freezxp/syslogx/backend/internal/transport/syslog"
	"github.com/prometheus/client_golang/prometheus"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("syslogx stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var configPath, httpAddress, storageEndpoint string
	var showVersion bool
	flag.StringVar(&configPath, "config", "config/syslogx.yaml", "path to YAML configuration")
	flag.StringVar(&httpAddress, "http-address", "", "override HTTP listen address")
	flag.StringVar(&storageEndpoint, "storage-endpoint", "", "override storage endpoint")
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.Parse()
	if showVersion {
		fmt.Println(version)
		return nil
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	cfg, err := config.Load(configPath, os.Environ(), config.Overrides{HTTPAddress: httpAddress, StorageEndpoint: storageEndpoint})
	if err != nil {
		return err
	}
	backend, err := victorialogs.New(cfg.Storage.Endpoint, cfg.Storage.Timeout, cfg.Storage.StreamFields)
	if err != nil {
		return err
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serviceCtx, serviceCancel := context.WithCancel(context.Background())
	defer serviceCancel()
	var controlStore *postgres.Store
	if cfg.ControlStore.Enabled {
		dsn, err := cfg.ControlDSN()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(rootCtx, cfg.ControlStore.Timeout)
		controlStore, err = postgres.Open(ctx, dsn)
		cancel()
		if err != nil {
			return err
		}
		defer controlStore.Close()
		ctx, cancel = context.WithTimeout(rootCtx, cfg.ControlStore.Timeout)
		err = controlStore.Migrate(ctx)
		cancel()
		if err != nil {
			return err
		}
	}

	m := metrics.New(prometheus.DefaultRegisterer)
	httpMetrics := metrics.NewHTTP(prometheus.DefaultRegisterer)
	accepting := &atomic.Bool{}
	pipeline, err := ingestion.NewPipeline(cfg.Ingestion, backend, m, accepting, logger)
	if err != nil {
		return err
	}
	pipeline.Start(serviceCtx, max(2, runtime.GOMAXPROCS(0)))
	sources := transport.NewManager(cfg.Ingestion.Sources, pipeline, m, logger)
	if err := sources.Start(serviceCtx); err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Timeout)
		defer cancel()
		_ = pipeline.Stop(shutdownCtx)
		return err
	}

	var controlChecker api.ControlChecker
	if controlStore != nil {
		controlChecker = controlStore
	}
	server := api.New(cfg.Server.HTTP, backend, controlChecker, cfg.ControlStore.Timeout, accepting, httpMetrics, logger)
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server started", "address", cfg.Server.HTTP.Address, "version", version)
		serverErrors <- server.ListenAndServe()
	}()

	var runErr error
	select {
	case <-rootCtx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			runErr = err
		}
	}
	accepting.Store(false)
	_ = sources.Close()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Shutdown.Timeout)
	defer cancel()
	pipelineErr := pipeline.Stop(shutdownCtx)
	serverErr := server.Shutdown(shutdownCtx)
	serviceCancel()
	return errors.Join(runErr, pipelineErr, serverErr)
}
