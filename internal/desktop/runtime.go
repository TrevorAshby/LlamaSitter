package desktop

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/configedit"
)

const (
	EnvConfigOverride       = "LLAMASITTER_DESKTOP_CONFIG"
	EnvAttachOnly           = "LLAMASITTER_DESKTOP_ATTACH_ONLY"
	EnvManaged              = "LLAMASITTER_DESKTOP_MANAGED"
	EnvLinuxDesktopApp      = "LLAMASITTER_LINUX_DESKTOP_APP"
	EnvNoDesktopAutoLaunch  = "LLAMASITTER_NO_DESKTOP_AUTO_LAUNCH"
	defaultLinuxConfigDir   = ".config"
	defaultLinuxStateParent = ".local/state"
)

type Paths struct {
	Platform              string `json:"platform"`
	ApplicationSupportDir string `json:"application_support_dir,omitempty"`
	ConfigDir             string `json:"config_dir,omitempty"`
	StateDir              string `json:"state_dir,omitempty"`
	Config                string `json:"config"`
	DB                    string `json:"db"`
	Logs                  string `json:"logs"`
	AppLog                string `json:"app_log"`
	BackendLog            string `json:"backend_log"`
	Autostart             string `json:"autostart,omitempty"`
}

type Runtime struct {
	Platform              string `json:"platform"`
	ApplicationSupportDir string `json:"application_support_dir,omitempty"`
	ConfigDir             string `json:"config_dir,omitempty"`
	StateDir              string `json:"state_dir,omitempty"`
	ConfigPath            string `json:"config_path"`
	DBPath                string `json:"db_path"`
	LogsPath              string `json:"logs_path"`
	AppLogPath            string `json:"app_log_path"`
	BackendLogPath        string `json:"backend_log_path"`
	AutostartPath         string `json:"autostart_path,omitempty"`
	ProxyListenAddr       string `json:"proxy_listen_addr"`
	UIListenAddr          string `json:"ui_listen_addr"`
	UIBaseURL             string `json:"ui_base_url"`
	ReadyURL              string `json:"ready_url"`
	AttachOnly            bool   `json:"attach_only"`
	BackendExecutable     string `json:"backend_executable"`
}

type AutostartStatus struct {
	Path    string `json:"path"`
	Enabled bool   `json:"enabled"`
}

func ManagedPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return pathsForOS(runtime.GOOS, home, os.Getenv)
}

func ResolveConfigPath(flagValue string, flagChanged bool) (string, error) {
	if flagChanged {
		return absExpandedPath(flagValue)
	}
	if override := ConfigOverrideFromEnv(); override != "" {
		return absExpandedPath(override)
	}
	paths, err := ManagedPaths()
	if err != nil {
		return "", err
	}
	return paths.Config, nil
}

func ResolveRuntime(configPath string, attachOnly bool, backendExecutable string) (Runtime, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Runtime{}, err
	}
	if strings.TrimSpace(backendExecutable) == "" {
		backendExecutable, _ = os.Executable()
	}
	return runtimeForOS(runtime.GOOS, home, os.Getenv, configPath, attachOnly, backendExecutable)
}

func ConfigOverrideFromEnv() string {
	value := strings.TrimSpace(os.Getenv(EnvConfigOverride))
	if value == "" {
		return ""
	}
	return filepath.Clean(value)
}

func AttachOnlyFromEnv() bool {
	return truthy(os.Getenv(EnvAttachOnly))
}

func IsGraphicalSession() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	return strings.TrimSpace(os.Getenv("DISPLAY")) != "" ||
		strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != ""
}

func LinuxDesktopExecutableCandidates() []string {
	candidates := make([]string, 0, 5)
	if override := strings.TrimSpace(os.Getenv(EnvLinuxDesktopApp)); override != "" {
		candidates = append(candidates, override)
	}
	if resolved, err := exec.LookPath("llamasitter-desktop"); err == nil && strings.TrimSpace(resolved) != "" {
		candidates = append(candidates, resolved)
	}
	candidates = append(candidates,
		"/usr/local/bin/llamasitter-desktop",
		"/usr/bin/llamasitter-desktop",
	)
	return uniqStrings(candidates)
}

func FirstExistingPath(paths []string) string {
	for _, candidate := range paths {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func AutostartState() (AutostartStatus, error) {
	paths, err := ManagedPaths()
	if err != nil {
		return AutostartStatus{}, err
	}
	if runtime.GOOS != "linux" {
		return AutostartStatus{}, fmt.Errorf("desktop autostart is only available on Linux")
	}
	_, err = os.Stat(paths.Autostart)
	return AutostartStatus{
		Path:    paths.Autostart,
		Enabled: err == nil,
	}, nil
}

func EnableAutostart(configPath, desktopExecutable string) (AutostartStatus, error) {
	paths, err := ManagedPaths()
	if err != nil {
		return AutostartStatus{}, err
	}
	if runtime.GOOS != "linux" {
		return AutostartStatus{}, fmt.Errorf("desktop autostart is only available on Linux")
	}
	if strings.TrimSpace(desktopExecutable) == "" {
		desktopExecutable = FirstExistingPath(LinuxDesktopExecutableCandidates())
	}
	if strings.TrimSpace(desktopExecutable) == "" {
		return AutostartStatus{}, fmt.Errorf("unable to find the Linux desktop companion executable")
	}
	if err := os.MkdirAll(filepath.Dir(paths.Autostart), 0o755); err != nil {
		return AutostartStatus{}, fmt.Errorf("create autostart dir: %w", err)
	}
	if err := os.WriteFile(paths.Autostart, []byte(LinuxAutostartEntry(desktopExecutable, configPath)), 0o644); err != nil {
		return AutostartStatus{}, fmt.Errorf("write autostart file: %w", err)
	}
	return AutostartStatus{Path: paths.Autostart, Enabled: true}, nil
}

func DisableAutostart() (AutostartStatus, error) {
	paths, err := ManagedPaths()
	if err != nil {
		return AutostartStatus{}, err
	}
	if runtime.GOOS != "linux" {
		return AutostartStatus{}, fmt.Errorf("desktop autostart is only available on Linux")
	}
	err = os.Remove(paths.Autostart)
	if err != nil && !os.IsNotExist(err) {
		return AutostartStatus{}, fmt.Errorf("remove autostart file: %w", err)
	}
	return AutostartStatus{Path: paths.Autostart, Enabled: false}, nil
}

func LinuxAutostartEntry(desktopExecutable, configPath string) string {
	execArgs := []string{shellQuote(desktopExecutable), "--mode=tray", "--attach-only"}
	if strings.TrimSpace(configPath) != "" {
		execArgs = append(execArgs, "--config", shellQuote(configPath))
	}
	return strings.TrimSpace(fmt.Sprintf(`
[Desktop Entry]
Type=Application
Version=1.0
Name=LlamaSitter Tray
Comment=Start the LlamaSitter tray agent
Exec=%s
Icon=llamasitter
Terminal=false
NoDisplay=true
Categories=Development;Utility;
StartupNotify=false
X-GNOME-Autostart-enabled=true
`, strings.Join(execArgs, " "))) + "\n"
}

func runtimeForOS(goos, home string, getenv func(string) string, configPath string, attachOnly bool, backendExecutable string) (Runtime, error) {
	paths, err := pathsForOS(goos, home, getenv)
	if err != nil {
		return Runtime{}, err
	}

	override := strings.TrimSpace(configPath)
	if override == "" {
		override = strings.TrimSpace(getenv(EnvConfigOverride))
	}
	if override != "" {
		override, err = absExpandedPath(override)
		if err != nil {
			return Runtime{}, err
		}
	}

	configPath = paths.Config
	if override != "" {
		configPath = filepath.Clean(override)
	}

	if err := ensureManagedRuntime(paths, configPath); err != nil {
		return Runtime{}, err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return Runtime{}, err
	}
	if len(cfg.Listeners) == 0 {
		return Runtime{}, fmt.Errorf("desktop config must define at least one listener")
	}

	uiBaseURL, err := baseURLForListenAddr(cfg.UI.ListenAddr)
	if err != nil {
		return Runtime{}, err
	}
	readyURL, err := url.JoinPath(uiBaseURL, "readyz")
	if err != nil {
		return Runtime{}, err
	}

	return Runtime{
		Platform:              paths.Platform,
		ApplicationSupportDir: paths.ApplicationSupportDir,
		ConfigDir:             paths.ConfigDir,
		StateDir:              paths.StateDir,
		ConfigPath:            configPath,
		DBPath:                cfg.Storage.SQLitePath,
		LogsPath:              paths.Logs,
		AppLogPath:            paths.AppLog,
		BackendLogPath:        paths.BackendLog,
		AutostartPath:         paths.Autostart,
		ProxyListenAddr:       cfg.Listeners[0].ListenAddr,
		UIListenAddr:          cfg.UI.ListenAddr,
		UIBaseURL:             uiBaseURL,
		ReadyURL:              readyURL,
		AttachOnly:            attachOnly,
		BackendExecutable:     backendExecutable,
	}, nil
}

func pathsForOS(goos, home string, getenv func(string) string) (Paths, error) {
	if strings.TrimSpace(home) == "" {
		return Paths{}, fmt.Errorf("user home directory is unavailable")
	}

	switch goos {
	case "darwin":
		appSupport := filepath.Join(home, "Library", "Application Support", "LlamaSitter")
		logs := filepath.Join(home, "Library", "Logs", "LlamaSitter")
		return Paths{
			Platform:              "darwin",
			ApplicationSupportDir: appSupport,
			Config:                filepath.Join(appSupport, "llamasitter.yaml"),
			DB:                    filepath.Join(appSupport, "llamasitter.db"),
			Logs:                  logs,
			AppLog:                filepath.Join(logs, "app.log"),
			BackendLog:            filepath.Join(logs, "backend.log"),
		}, nil
	case "linux":
		configRoot := strings.TrimSpace(getenv("XDG_CONFIG_HOME"))
		if configRoot == "" {
			configRoot = filepath.Join(home, defaultLinuxConfigDir)
		}
		stateRoot := strings.TrimSpace(getenv("XDG_STATE_HOME"))
		if stateRoot == "" {
			stateRoot = filepath.Join(home, defaultLinuxStateParent)
		}
		configDir := filepath.Join(configRoot, "llamasitter")
		stateDir := filepath.Join(stateRoot, "llamasitter")
		logsDir := filepath.Join(stateDir, "logs")
		return Paths{
			Platform:   "linux",
			ConfigDir:  configDir,
			StateDir:   stateDir,
			Config:     filepath.Join(configDir, "llamasitter.yaml"),
			DB:         filepath.Join(stateDir, "llamasitter.db"),
			Logs:       logsDir,
			AppLog:     filepath.Join(logsDir, "app.log"),
			BackendLog: filepath.Join(logsDir, "backend.log"),
			Autostart:  filepath.Join(configRoot, "autostart", "com.trevorashby.LlamaSitter.Tray.desktop"),
		}, nil
	default:
		return Paths{}, fmt.Errorf("desktop helpers are only available on macOS and Linux")
	}
}

func ensureManagedRuntime(paths Paths, configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.DB), 0o755); err != nil {
		return fmt.Errorf("create database dir: %w", err)
	}
	if err := os.MkdirAll(paths.Logs, 0o755); err != nil {
		return fmt.Errorf("create logs dir: %w", err)
	}

	if _, err := os.Stat(configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config: %w", err)
	}

	if configPath != paths.Config {
		return fmt.Errorf("configured LlamaSitter config does not exist at %s", configPath)
	}

	doc := configedit.NewDefault()
	if err := doc.SetStorageSQLitePath(paths.DB); err != nil {
		return fmt.Errorf("set storage path: %w", err)
	}

	switch paths.Platform {
	case "darwin":
		if err := doc.SetListenerTag("default", "client_type", "dock-app"); err != nil {
			return fmt.Errorf("set client_type: %w", err)
		}
		if err := doc.SetListenerTag("default", "client_instance", "macos"); err != nil {
			return fmt.Errorf("set client_instance: %w", err)
		}
	case "linux":
		if err := doc.SetListenerTag("default", "client_type", "desktop-app"); err != nil {
			return fmt.Errorf("set client_type: %w", err)
		}
		if err := doc.SetListenerTag("default", "client_instance", "linux"); err != nil {
			return fmt.Errorf("set client_instance: %w", err)
		}
	}

	if err := doc.WriteAtomic(configPath); err != nil {
		return fmt.Errorf("write default desktop config: %w", err)
	}
	return nil
}

func baseURLForListenAddr(listenAddr string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(listenAddr))
	if err != nil {
		return "", fmt.Errorf("invalid listen address %q: %w", listenAddr, err)
	}
	if host == "" || port == "" {
		return "", fmt.Errorf("invalid listen address %q", listenAddr)
	}
	return (&url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, port),
	}).String(), nil
}

func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	escaped := strings.ReplaceAll(value, `'`, `'\''`)
	return "'" + escaped + "'"
}

func uniqStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func absExpandedPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}
	return filepath.Abs(path)
}
