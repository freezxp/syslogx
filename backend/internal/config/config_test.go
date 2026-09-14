package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPrecedenceAndDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("server:\n  http:\n    address: ':7000'\nstorage:\n  endpoint: http://logs:9428\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path, []string{"SYSLOGX_HTTP_ADDRESS=:7001"}, Overrides{HTTPAddress: ":7002"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.HTTP.Address != ":7002" || cfg.Storage.Type != "victorialogs" || cfg.Ingestion.Queue.MaxEvents == 0 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, nil, Overrides{}); err == nil {
		t.Fatal("expected unknown key error")
	}
}

func TestLoadRejectsDuplicateSource(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte("ingestion:\n  sources:\n  - {id: same, protocol: udp, address: ':1'}\n  - {id: same, protocol: tcp, address: ':2'}\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path, nil, Overrides{}); err == nil {
		t.Fatal("expected duplicate source error")
	}
}
