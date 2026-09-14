package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Ingestion    IngestionConfig    `yaml:"ingestion"`
	Storage      StorageConfig      `yaml:"storage"`
	ControlStore ControlStoreConfig `yaml:"control_store"`
	Shutdown     ShutdownConfig     `yaml:"shutdown"`
}

type ServerConfig struct {
	HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
	Address               string        `yaml:"address"`
	ReadTimeout           time.Duration `yaml:"-"`
	WriteTimeout          time.Duration `yaml:"-"`
	IdleTimeout           time.Duration `yaml:"-"`
	ReadHeaderTimeout     time.Duration `yaml:"-"`
	ReadTimeoutText       string        `yaml:"read_timeout"`
	WriteTimeoutText      string        `yaml:"write_timeout"`
	IdleTimeoutText       string        `yaml:"idle_timeout"`
	ReadHeaderTimeoutText string        `yaml:"read_header_timeout"`
}

type IngestionConfig struct {
	Queue   QueueConfig    `yaml:"queue"`
	Batch   BatchConfig    `yaml:"batch"`
	Retry   RetryConfig    `yaml:"retry"`
	Sources []SourceConfig `yaml:"sources"`
}

type QueueConfig struct {
	MaxEvents          int           `yaml:"max_events"`
	MaxBytes           int64         `yaml:"max_bytes"`
	EnqueueTimeout     time.Duration `yaml:"-"`
	EnqueueTimeoutText string        `yaml:"enqueue_timeout"`
}

type BatchConfig struct {
	MaxEvents  int           `yaml:"max_events"`
	MaxBytes   int64         `yaml:"max_bytes"`
	MaxAge     time.Duration `yaml:"-"`
	MaxAgeText string        `yaml:"max_age"`
}

type RetryConfig struct {
	MaxAttempts        int           `yaml:"max_attempts"`
	InitialBackoff     time.Duration `yaml:"-"`
	MaxBackoff         time.Duration `yaml:"-"`
	InitialBackoffText string        `yaml:"initial_backoff"`
	MaxBackoffText     string        `yaml:"max_backoff"`
}

type SourceConfig struct {
	ID              string `yaml:"id"`
	Name            string `yaml:"name"`
	Protocol        string `yaml:"protocol"`
	Address         string `yaml:"address"`
	Parser          string `yaml:"parser"`
	Enabled         bool   `yaml:"enabled"`
	MaxMessageBytes int    `yaml:"max_message_bytes"`
	MaxConnections  int    `yaml:"max_connections"`
	Framing         string `yaml:"framing"`
	TenantID        string `yaml:"tenant_id"`
	Timezone        string `yaml:"timezone"`
}

type StorageConfig struct {
	Type         string        `yaml:"type"`
	Endpoint     string        `yaml:"endpoint"`
	Timeout      time.Duration `yaml:"-"`
	TimeoutText  string        `yaml:"timeout"`
	StreamFields []string      `yaml:"stream_fields"`
}

type ControlStoreConfig struct {
	Enabled     bool          `yaml:"enabled"`
	DSN         string        `yaml:"dsn"`
	DSNFile     string        `yaml:"dsn_file"`
	Timeout     time.Duration `yaml:"-"`
	TimeoutText string        `yaml:"timeout"`
}

type ShutdownConfig struct {
	Timeout     time.Duration `yaml:"-"`
	TimeoutText string        `yaml:"timeout"`
}

type Overrides struct {
	HTTPAddress     string
	StorageEndpoint string
}

func Default() Config {
	return Config{
		Server: ServerConfig{HTTP: HTTPConfig{Address: ":8080", ReadTimeoutText: "15s", WriteTimeoutText: "30s", IdleTimeoutText: "60s", ReadHeaderTimeoutText: "5s"}},
		Ingestion: IngestionConfig{
			Queue: QueueConfig{MaxEvents: 100_000, MaxBytes: 256 << 20, EnqueueTimeoutText: "25ms"},
			Batch: BatchConfig{MaxEvents: 1_000, MaxBytes: 4 << 20, MaxAgeText: "1s"},
			Retry: RetryConfig{MaxAttempts: 5, InitialBackoffText: "100ms", MaxBackoffText: "5s"},
		},
		Storage:      StorageConfig{Type: "victorialogs", Endpoint: "http://127.0.0.1:9428", TimeoutText: "10s", StreamFields: []string{"tenant_id", "source_type", "hostname", "app_name"}},
		ControlStore: ControlStoreConfig{Enabled: false, TimeoutText: "5s"},
		Shutdown:     ShutdownConfig{TimeoutText: "20s"},
	}
}

func Load(path string, environ []string, overrides Overrides) (Config, error) {
	cfg := Default()
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return Config{}, fmt.Errorf("open config: %w", err)
		}
		defer f.Close()
		dec := yaml.NewDecoder(io.LimitReader(f, 4<<20))
		dec.KnownFields(true)
		if err := dec.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("decode config: %w", err)
		}
	}
	if err := applyEnv(&cfg, environ); err != nil {
		return Config{}, err
	}
	if overrides.HTTPAddress != "" {
		cfg.Server.HTTP.Address = overrides.HTTPAddress
	}
	if overrides.StorageEndpoint != "" {
		cfg.Storage.Endpoint = overrides.StorageEndpoint
	}
	if err := cfg.finalize(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config, environ []string) error {
	values := make(map[string]string, len(environ))
	for _, item := range environ {
		k, v, ok := strings.Cut(item, "=")
		if ok {
			values[k] = v
		}
	}
	set := func(name string, dst *string) {
		if v, ok := values[name]; ok {
			*dst = v
		}
	}
	set("SYSLOGX_HTTP_ADDRESS", &cfg.Server.HTTP.Address)
	set("SYSLOGX_STORAGE_ENDPOINT", &cfg.Storage.Endpoint)
	set("SYSLOGX_CONTROL_STORE_DSN", &cfg.ControlStore.DSN)
	set("SYSLOGX_CONTROL_STORE_DSN_FILE", &cfg.ControlStore.DSNFile)
	if v, ok := values["SYSLOGX_CONTROL_STORE_ENABLED"]; ok {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return fmt.Errorf("SYSLOGX_CONTROL_STORE_ENABLED: %w", err)
		}
		cfg.ControlStore.Enabled = b
	}
	return nil
}

func (c *Config) finalize() error {
	parseDuration := func(name, value string, dst *time.Duration) error {
		d, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if d <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
		*dst = d
		return nil
	}
	for _, item := range []struct {
		name, value string
		dst         *time.Duration
	}{
		{"server.http.read_timeout", c.Server.HTTP.ReadTimeoutText, &c.Server.HTTP.ReadTimeout},
		{"server.http.write_timeout", c.Server.HTTP.WriteTimeoutText, &c.Server.HTTP.WriteTimeout},
		{"server.http.idle_timeout", c.Server.HTTP.IdleTimeoutText, &c.Server.HTTP.IdleTimeout},
		{"server.http.read_header_timeout", c.Server.HTTP.ReadHeaderTimeoutText, &c.Server.HTTP.ReadHeaderTimeout},
		{"ingestion.queue.enqueue_timeout", c.Ingestion.Queue.EnqueueTimeoutText, &c.Ingestion.Queue.EnqueueTimeout},
		{"ingestion.batch.max_age", c.Ingestion.Batch.MaxAgeText, &c.Ingestion.Batch.MaxAge},
		{"ingestion.retry.initial_backoff", c.Ingestion.Retry.InitialBackoffText, &c.Ingestion.Retry.InitialBackoff},
		{"ingestion.retry.max_backoff", c.Ingestion.Retry.MaxBackoffText, &c.Ingestion.Retry.MaxBackoff},
		{"storage.timeout", c.Storage.TimeoutText, &c.Storage.Timeout},
		{"control_store.timeout", c.ControlStore.TimeoutText, &c.ControlStore.Timeout},
		{"shutdown.timeout", c.Shutdown.TimeoutText, &c.Shutdown.Timeout},
	} {
		if err := parseDuration(item.name, item.value, item.dst); err != nil {
			return err
		}
	}
	if c.Server.HTTP.Address == "" {
		return errors.New("server.http.address is required")
	}
	if c.Ingestion.Queue.MaxEvents <= 0 || c.Ingestion.Queue.MaxBytes <= 0 {
		return errors.New("ingestion queue limits must be positive")
	}
	if c.Ingestion.Batch.MaxEvents <= 0 || c.Ingestion.Batch.MaxBytes <= 0 {
		return errors.New("ingestion batch limits must be positive")
	}
	if c.Ingestion.Batch.MaxEvents > c.Ingestion.Queue.MaxEvents || c.Ingestion.Batch.MaxBytes > c.Ingestion.Queue.MaxBytes {
		return errors.New("batch limits cannot exceed queue limits")
	}
	if c.Ingestion.Retry.MaxAttempts <= 0 || c.Ingestion.Retry.InitialBackoff > c.Ingestion.Retry.MaxBackoff {
		return errors.New("invalid retry policy")
	}
	if c.Storage.Type != "victorialogs" {
		return fmt.Errorf("unsupported storage type %q", c.Storage.Type)
	}
	u, err := url.Parse(c.Storage.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("storage.endpoint must be an absolute http(s) URL")
	}
	seen := map[string]bool{}
	for i := range c.Ingestion.Sources {
		s := &c.Ingestion.Sources[i]
		if s.ID == "" || seen[s.ID] {
			return fmt.Errorf("source %d has empty or duplicate id", i)
		}
		seen[s.ID] = true
		if s.TenantID == "" {
			s.TenantID = "default"
		}
		if s.Parser == "" {
			s.Parser = "auto"
		}
		if s.MaxMessageBytes == 0 {
			s.MaxMessageBytes = 64 << 10
		}
		if s.MaxConnections == 0 {
			s.MaxConnections = 1024
		}
		if s.Framing == "" {
			s.Framing = "auto"
		}
		if s.Timezone == "" {
			s.Timezone = "UTC"
		}
		if s.Protocol != "udp" && s.Protocol != "tcp" {
			return fmt.Errorf("source %q has unsupported protocol %q", s.ID, s.Protocol)
		}
		if s.Address == "" || s.MaxMessageBytes <= 0 || s.MaxConnections <= 0 {
			return fmt.Errorf("source %q has invalid limits/address", s.ID)
		}
		if s.Parser != "auto" && s.Parser != "rfc3164" && s.Parser != "rfc5424" {
			return fmt.Errorf("source %q has unsupported parser %q", s.ID, s.Parser)
		}
		if s.Framing != "auto" && s.Framing != "newline" && s.Framing != "octet_counting" {
			return fmt.Errorf("source %q has unsupported framing %q", s.ID, s.Framing)
		}
		if _, err := time.LoadLocation(s.Timezone); err != nil {
			return fmt.Errorf("source %q timezone: %w", s.ID, err)
		}
	}
	if c.ControlStore.Enabled && c.ControlStore.DSN == "" && c.ControlStore.DSNFile == "" {
		return errors.New("control_store dsn or dsn_file is required when enabled")
	}
	return nil
}

func (c Config) ControlDSN() (string, error) {
	if c.ControlStore.DSN != "" {
		return c.ControlStore.DSN, nil
	}
	if c.ControlStore.DSNFile == "" {
		return "", nil
	}
	b, err := os.ReadFile(c.ControlStore.DSNFile)
	if err != nil {
		return "", fmt.Errorf("read control store dsn: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
