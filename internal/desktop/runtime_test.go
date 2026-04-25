package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsForDarwin(t *testing.T) {
	t.Parallel()

	paths, err := pathsForOS("darwin", "/Users/tester", func(string) string { return "" })
	if err != nil {
		t.Fatalf("pathsForOS: %v", err)
	}

	if got, want := paths.Config, "/Users/tester/Library/Application Support/LlamaSitter/llamasitter.yaml"; got != want {
		t.Fatalf("config = %q, want %q", got, want)
	}
	if got, want := paths.DB, "/Users/tester/Library/Application Support/LlamaSitter/llamasitter.db"; got != want {
		t.Fatalf("db = %q, want %q", got, want)
	}
	if got, want := paths.Logs, "/Users/tester/Library/Logs/LlamaSitter"; got != want {
		t.Fatalf("logs = %q, want %q", got, want)
	}
}

func TestPathsForLinuxRespectsXDG(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"XDG_CONFIG_HOME": "/tmp/config-home",
		"XDG_STATE_HOME":  "/tmp/state-home",
	}
	paths, err := pathsForOS("linux", "/home/tester", func(key string) string {
		return env[key]
	})
	if err != nil {
		t.Fatalf("pathsForOS: %v", err)
	}

	if got, want := paths.Config, "/tmp/config-home/llamasitter/llamasitter.yaml"; got != want {
		t.Fatalf("config = %q, want %q", got, want)
	}
	if got, want := paths.DB, "/tmp/state-home/llamasitter/llamasitter.db"; got != want {
		t.Fatalf("db = %q, want %q", got, want)
	}
	if got, want := paths.Autostart, "/tmp/config-home/autostart/com.trevorashby.LlamaSitter.Tray.desktop"; got != want {
		t.Fatalf("autostart = %q, want %q", got, want)
	}
}

func TestRuntimeForLinuxCreatesManagedConfig(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	env := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
	}
	runtimeInfo, err := runtimeForOS("linux", home, func(key string) string {
		return env[key]
	}, "", true, "/usr/bin/llamasitter")
	if err != nil {
		t.Fatalf("runtimeForOS: %v", err)
	}

	if !runtimeInfo.AttachOnly {
		t.Fatalf("expected attach_only=true")
	}
	if got, want := runtimeInfo.ConfigPath, filepath.Join(home, ".config", "llamasitter", "llamasitter.yaml"); got != want {
		t.Fatalf("config path = %q, want %q", got, want)
	}

	raw, err := os.ReadFile(runtimeInfo.ConfigPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `sqlite_path: "`+filepath.Join(home, ".local", "state", "llamasitter", "llamasitter.db")+`"`) {
		t.Fatalf("expected managed sqlite path in config, got:\n%s", text)
	}
	if !strings.Contains(text, `client_instance: "linux"`) {
		t.Fatalf("expected linux desktop defaults, got:\n%s", text)
	}
}

func TestRuntimeForOverrideRequiresExistingFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}

	_, err := runtimeForOS("linux", home, func(string) string { return "" }, filepath.Join(root, "missing.yaml"), false, "/usr/bin/llamasitter")
	if err == nil {
		t.Fatalf("expected missing override to fail")
	}
}

func TestLinuxAutostartEntryIncludesAttachOnlyAndOptionalConfig(t *testing.T) {
	t.Parallel()

	entry := LinuxAutostartEntry("/usr/bin/llamasitter-desktop", "/tmp/llamasitter.yaml")
	if !strings.Contains(entry, "Exec='/usr/bin/llamasitter-desktop' --mode=tray --attach-only --config '/tmp/llamasitter.yaml'") {
		t.Fatalf("unexpected autostart entry:\n%s", entry)
	}

	entry = LinuxAutostartEntry("/usr/bin/llamasitter-desktop", "")
	if strings.Contains(entry, "--config") {
		t.Fatalf("expected config flag to be omitted when empty:\n%s", entry)
	}
}
