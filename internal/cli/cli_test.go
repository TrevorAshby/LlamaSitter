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
	"time"

	"github.com/trevorashby/llamasitter/internal/desktop"
	"github.com/trevorashby/llamasitter/internal/model"
	"github.com/trevorashby/llamasitter/internal/storage"
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

	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("desktop helper is only available on macOS and Linux")
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"desktop", "config", "path"}, testLogger(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}
	paths, err := desktop.ManagedPaths()
	if err != nil {
		t.Fatalf("managed paths: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != paths.Config {
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

func TestFirstExistingBundlePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	existing := filepath.Join(dir, "existing")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatalf("mkdir existing: %v", err)
	}

	got := firstExistingBundlePath([]string{"", missing, existing})
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

func TestStatsCommandRendersExpandedOverview(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "llamasitter.yaml")
	dbPath := filepath.Join(dir, "llamasitter.db")

	config := []byte(`listeners:
  - name: default
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"
    default_tags:
      client_type: "desktop-app"
      client_instance: "macos"

storage:
  sqlite_path: "` + dbPath + `"

ui:
  enabled: true
  listen_addr: "127.0.0.1:11438"
`)
	if err := os.WriteFile(configPath, config, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	store, err := storage.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UTC()
	events := []*model.RequestEvent{
		{
			RequestID:               "req-1",
			ListenerName:            "default",
			StartedAt:               now.Add(-30 * time.Minute),
			FinishedAt:              now.Add(-29*time.Minute - 500*time.Millisecond),
			Method:                  "POST",
			Endpoint:                "/api/chat",
			Model:                   "llama3",
			HTTPStatus:              200,
			Success:                 true,
			PromptTokens:            120,
			OutputTokens:            280,
			TotalTokens:             400,
			RequestDurationMs:       1500,
			UpstreamTotalDurationMs: 1300,
			Identity: model.Identity{
				ClientType:     "desktop-app",
				ClientInstance: "macos",
				AgentName:      "planner",
				SessionID:      "session-a",
			},
		},
		{
			RequestID:               "req-2",
			ListenerName:            "default",
			StartedAt:               now.Add(-10 * time.Minute),
			FinishedAt:              now.Add(-9*time.Minute - 250*time.Millisecond),
			Method:                  "POST",
			Endpoint:                "/api/chat",
			Model:                   "mistral",
			HTTPStatus:              502,
			Success:                 false,
			PromptTokens:            40,
			OutputTokens:            0,
			TotalTokens:             40,
			RequestDurationMs:       2250,
			UpstreamTotalDurationMs: 0,
			ErrorMessage:            "upstream unavailable",
			Identity: model.Identity{
				ClientType:     "desktop-app",
				ClientInstance: "macos",
				AgentName:      "reviewer",
				SessionID:      "session-b",
			},
		},
	}

	for _, event := range events {
		if err := store.InsertRequest(ctx, event); err != nil {
			t.Fatalf("insert request %s: %v", event.RequestID, err)
		}
	}

	t.Setenv("COLUMNS", "79")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := Execute(context.Background(), []string{"--config", configPath, "stats"}, testLogger(), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, stderr.String())
	}

	output := stdout.String()
	for _, needle := range []string{
		"LlamaSitter Stats",
		"Overview",
		"Recent Windows",
		"Daily Trend (Last 7d)",
		"Top Breakdown",
		"Listeners",
		"Top Sessions",
		"Recent Requests",
		"planner",
		"reviewer",
	} {
		if !strings.Contains(output, needle) {
			t.Fatalf("expected stats output to contain %q, got:\n%s", needle, output)
		}
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
