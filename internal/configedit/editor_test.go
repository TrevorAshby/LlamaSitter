package configedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trevorashby/llamasitter/internal/config"
)

func TestDocumentAddListenerPreservesCommentAndValidates(t *testing.T) {
	t.Parallel()

	doc, err := Parse([]byte(`
listeners:
  - name: default
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"

# keep this comment
storage:
  sqlite_path: "~/.llamasitter/llamasitter.db"

ui:
  enabled: true
  listen_addr: "127.0.0.1:11438"
`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if err := doc.AddListener(listener("openwebui", "127.0.0.1:11436")); err != nil {
		t.Fatalf("add listener: %v", err)
	}

	raw, err := doc.Bytes()
	if err != nil {
		t.Fatalf("bytes: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "# keep this comment") {
		t.Fatalf("expected comment to be preserved, got:\n%s", text)
	}
	if !strings.Contains(text, `- name: openwebui`) {
		t.Fatalf("expected new listener in output, got:\n%s", text)
	}

	cfg, err := doc.Config()
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if len(cfg.Listeners) != 2 {
		t.Fatalf("expected 2 listeners, got %d", len(cfg.Listeners))
	}
}

func TestDocumentRemoveLastListenerFails(t *testing.T) {
	t.Parallel()

	doc := NewDefault()
	if err := doc.RemoveListener("default"); err == nil {
		t.Fatalf("expected remove last listener error")
	}
}

func TestDocumentSettersAndWriteAtomic(t *testing.T) {
	t.Parallel()

	doc := NewDefault()
	if err := doc.SetUIListenAddr("127.0.0.1:11439"); err != nil {
		t.Fatalf("set ui addr: %v", err)
	}
	if err := doc.SetPersistBodies(true); err != nil {
		t.Fatalf("set persist: %v", err)
	}
	if err := doc.AddRedactHeader("x-api-key"); err != nil {
		t.Fatalf("add redact header: %v", err)
	}
	if err := doc.SetListenerTag("default", "client_type", "desktop"); err != nil {
		t.Fatalf("set listener tag: %v", err)
	}

	cfg, err := doc.Config()
	if err != nil {
		t.Fatalf("config validate: %v", err)
	}
	if cfg.UI.ListenAddr != "127.0.0.1:11439" {
		t.Fatalf("unexpected ui listen addr: %s", cfg.UI.ListenAddr)
	}
	if !cfg.Privacy.PersistBodies {
		t.Fatalf("expected persist bodies true")
	}

	path := filepath.Join(t.TempDir(), "llamasitter.yaml")
	if err := doc.WriteAtomic(path); err != nil {
		t.Fatalf("write atomic: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(raw), "x-api-key") {
		t.Fatalf("expected redacted header in written config, got:\n%s", raw)
	}
}

func listener(name, addr string) config.ListenerConfig {
	return config.ListenerConfig{
		Name:        name,
		ListenAddr:  addr,
		UpstreamURL: "http://127.0.0.1:11434",
	}
}
