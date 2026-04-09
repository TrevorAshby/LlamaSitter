package cli

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExecuteRootHelpWithoutArgs(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), nil, testLogger(), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), "config      Inspect, validate, and safely mutate LlamaSitter config files") {
		t.Fatalf("expected root help in stdout, got:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got:\n%s", stderr.String())
	}
}

func TestConfigListenerAddDryRunLeavesFileUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "llamasitter.yaml")
	original := []byte(`listeners:
  - name: default
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"

storage:
  sqlite_path: "` + filepath.Join(dir, "data.db") + `"

ui:
  enabled: true
  listen_addr: "127.0.0.1:11438"
`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{
		"--config", path,
		"config", "listener", "add",
		"--name", "openwebui",
		"--listen-addr", "127.0.0.1:11436",
		"--upstream-url", "http://127.0.0.1:11434",
		"--dry-run",
	}, testLogger(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `name: openwebui`) {
		t.Fatalf("expected dry-run output to include new listener, got:\n%s", stdout.String())
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back config: %v", err)
	}
	if string(current) != string(original) {
		t.Fatalf("expected dry-run not to modify config file")
	}
}

func TestDesktopConfigPathCommand(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "darwin" {
		t.Skip("desktop helper is macOS-only")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"desktop", "config", "path"}, testLogger(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Library/Application Support/LlamaSitter/llamasitter.yaml") {
		t.Fatalf("unexpected path output: %s", stdout.String())
	}
}

func TestVersionCommand(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"version"}, testLogger(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "version\t") {
		t.Fatalf("unexpected version output: %s", stdout.String())
	}
}

func TestGenerateReferenceDocs(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "reference")
	if err := GenerateReferenceDocs(dir); err != nil {
		t.Fatalf("generate docs: %v", err)
	}

	for _, name := range []string{
		"llamasitter.md",
		"llamasitter_config_listener_add.md",
		"llamasitter_completion.md",
		"llamasitter_version.md",
	} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected generated doc %s: %v", name, err)
		}
	}
}

func TestFirstExistingPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	existing := filepath.Join(dir, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatalf("mkdir existing: %v", err)
	}

	got := firstExistingPath([]string{"", missing, existing})
	if got != existing {
		t.Fatalf("expected %s, got %s", existing, got)
	}
}

func TestDesktopCompanionBundleCandidatesHonorsOverride(t *testing.T) {
	override := "/tmp/LlamaSitterMenu.app"
	t.Setenv("LLAMASITTER_MENU_AGENT_APP", override)

	candidates := desktopCompanionBundleCandidates()
	if len(candidates) == 0 || candidates[0] != override {
		t.Fatalf("expected override candidate first, got %#v", candidates)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
