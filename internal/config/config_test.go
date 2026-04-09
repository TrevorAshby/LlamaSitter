package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "llamasitter.yaml")
	content := `
listeners:
  - name: alpha
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"
storage:
  sqlite_path: "` + filepath.Join(dir, "data.db") + `"
`

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if len(cfg.Listeners) != 1 {
		t.Fatalf("expected 1 listener, got %d", len(cfg.Listeners))
	}
	if !cfg.UI.Enabled {
		t.Fatalf("expected UI to be enabled by default")
	}
	if cfg.UI.ListenAddr != "127.0.0.1:11438" {
		t.Fatalf("unexpected UI listen address: %s", cfg.UI.ListenAddr)
	}
	if len(cfg.Privacy.RedactHeaders) == 0 {
		t.Fatalf("expected default redact headers")
	}
	if !strings.HasSuffix(cfg.Storage.SQLitePath, "data.db") {
		t.Fatalf("expected expanded sqlite path, got %s", cfg.Storage.SQLitePath)
	}
}

func TestValidateRejectsDuplicateListenerNames(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Listeners: []ListenerConfig{
			{Name: "dup", ListenAddr: "127.0.0.1:11435", UpstreamURL: "http://127.0.0.1:11434"},
			{Name: "dup", ListenAddr: "127.0.0.1:11436", UpstreamURL: "http://127.0.0.1:11434"},
		},
		Storage: StorageConfig{SQLitePath: "/tmp/llamasitter.db"},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected duplicate name validation error")
	}
}
