package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/app"
	"github.com/trevorashby/llamasitter/internal/desktop"
	"github.com/trevorashby/llamasitter/internal/model"
)

type doctorListenerStatus struct {
	Name               string `json:"name" yaml:"name"`
	ListenAddr         string `json:"listen_addr" yaml:"listen_addr"`
	UpstreamVersionURL string `json:"upstream_version_url" yaml:"upstream_version_url"`
	UpstreamStatus     int    `json:"upstream_status" yaml:"upstream_status"`
	OK                 bool   `json:"ok" yaml:"ok"`
	Error              string `json:"error,omitempty" yaml:"error,omitempty"`
}

type doctorResult struct {
	ConfigPath   string                 `json:"config_path" yaml:"config_path"`
	StoragePath  string                 `json:"storage_path" yaml:"storage_path"`
	StorageOK    bool                   `json:"storage_ok" yaml:"storage_ok"`
	UIEnabled    bool                   `json:"ui_enabled" yaml:"ui_enabled"`
	UIListenAddr string                 `json:"ui_listen_addr,omitempty" yaml:"ui_listen_addr,omitempty"`
	Listeners    []doctorListenerStatus `json:"listeners" yaml:"listeners"`
}

func newServeCommand(_ context.Context, logger *slog.Logger, opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the proxy, storage, API, and dashboard services",
		Long: "Start the full LlamaSitter runtime using the selected config file. " +
			"This opens the configured SQLite store, starts all proxy listeners, serves the local API, and enables the dashboard UI when it is configured.",
		Args:  noArgs,
		Example: "  llamasitter serve --config llamasitter.yaml\n" +
			"  llamasitter serve --config /Users/me/Library/Application Support/LlamaSitter/llamasitter.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			runCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer stop()

			maybeLaunchDesktopCompanion(runCtx, opts.ConfigPath, configFlagChanged(cmd), logger)

			if err := app.Run(runCtx, cfg, logger); err != nil {
				return commandErrorf("%v", err)
			}
			return nil
		},
	}
}

func maybeLaunchDesktopCompanion(ctx context.Context, configPath string, _ bool, logger *slog.Logger) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return
	}
	if os.Getenv(desktop.EnvManaged) != "" || os.Getenv(desktop.EnvNoDesktopAutoLaunch) == "1" {
		return
	}
	if runtime.GOOS == "linux" && !desktop.IsGraphicalSession() {
		return
	}

	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		if logger != nil {
			logger.Warn("desktop companion auto-launch skipped", "reason", err.Error())
		}
		return
	}

	go func() {
		timer := time.NewTimer(750 * time.Millisecond)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		if runtime.GOOS == "darwin" {
			companionBundlePath := firstExistingBundlePath(desktopCompanionBundleCandidates())
			if companionBundlePath == "" {
				if logger != nil {
					logger.Info("desktop companion not found; continuing without menu icon", "config", resolvedConfigPath)
				}
				return
			}

			cmd := exec.Command("/usr/bin/open",
				"-g",
				"-j",
				companionBundlePath,
				"--args",
				"--config",
				resolvedConfigPath,
				"--attach-only",
			)
			cmd.Stdout = io.Discard
			cmd.Stderr = io.Discard
			if err := cmd.Start(); err != nil {
				if logger != nil {
					logger.Warn("desktop companion auto-launch failed", "bundle", companionBundlePath, "error", err.Error())
				}
				return
			}
			if cmd.Process != nil {
				_ = cmd.Process.Release()
			}
			return
		}

		companionPath := desktop.FirstExistingPath(desktop.LinuxDesktopExecutableCandidates())
		if companionPath == "" {
			if logger != nil {
				logger.Info("desktop companion not found; continuing without tray agent", "config", resolvedConfigPath)
			}
			return
		}

		cmd := exec.Command(companionPath,
			"--mode=tray",
			"--config",
			resolvedConfigPath,
			"--attach-only",
		)
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			if logger != nil {
				logger.Warn("desktop companion auto-launch failed", "binary", companionPath, "error", err.Error())
			}
			return
		}
		if cmd.Process != nil {
			_ = cmd.Process.Release()
		}
	}()
}

func desktopCompanionBundleCandidates() []string {
	candidates := make([]string, 0, 4)
	if override := strings.TrimSpace(os.Getenv("LLAMASITTER_MENU_AGENT_APP")); override != "" {
		candidates = append(candidates, override)
	}

	paths := []string{"/Applications/LlamaSitter.app"}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		paths = append(paths, filepath.Join(home, "Applications", "LlamaSitter.app"))
	}

	for _, appPath := range paths {
		candidates = append(candidates, filepath.Join(appPath, "Contents", "Library", "LoginItems", "LlamaSitterMenu.app"))
	}
	return candidates
}

func firstExistingBundlePath(paths []string) string {
	for _, candidate := range paths {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func newDoctorCommand(_ context.Context, logger *slog.Logger, opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Validate config, storage, listener upstreams, and UI settings",
		Long: "Check that the selected config file is valid, that storage can be opened, and that each configured listener can reach its upstream Ollama version endpoint. " +
			"Use this before starting the service or in scripts that need a fast health check.",
		Args:  noArgs,
		Example: "  llamasitter doctor --config llamasitter.yaml\n" +
			"  llamasitter doctor --config /Users/me/Library/Application Support/LlamaSitter/llamasitter.yaml --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, resolved, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("config: %v", err)
			}

			result := doctorResult{
				ConfigPath:   resolved,
				StoragePath:  cfg.Storage.SQLitePath,
				UIEnabled:    cfg.UI.Enabled,
				UIListenAddr: cfg.UI.ListenAddr,
			}

			store, err := app.OpenStore(cmd.Context(), cfg)
			if err != nil {
				return commandErrorf("storage: %v", err)
			}
			result.StorageOK = true
			defer store.Close()

			client := &http.Client{Timeout: 3 * time.Second}
			exitCode := 0
			for _, listener := range cfg.Listeners {
				status := doctorListenerStatus{
					Name:       listener.Name,
					ListenAddr: listener.ListenAddr,
					OK:         true,
				}

				versionURL, err := versionEndpoint(listener.UpstreamURL)
				if err != nil {
					status.OK = false
					status.Error = fmt.Sprintf("invalid upstream url: %v", err)
					result.Listeners = append(result.Listeners, status)
					exitCode = 1
					continue
				}
				status.UpstreamVersionURL = versionURL

				resp, err := client.Get(versionURL)
				if err != nil {
					status.OK = false
					status.Error = fmt.Sprintf("upstream unreachable: %v", err)
					result.Listeners = append(result.Listeners, status)
					exitCode = 1
					continue
				}
				status.UpstreamStatus = resp.StatusCode
				_ = resp.Body.Close()
				result.Listeners = append(result.Listeners, status)
			}

			switch format {
			case outputJSON:
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return commandErrorf("%v", err)
				}
			case outputYAML:
				if err := writeYAML(cmd.OutOrStdout(), result); err != nil {
					return commandErrorf("%v", err)
				}
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "config: ok (%s)\n", result.ConfigPath)
				fmt.Fprintf(cmd.OutOrStdout(), "storage: %s\n", result.StoragePath)
				fmt.Fprintln(cmd.OutOrStdout(), "storage: ok")
				for _, listener := range result.Listeners {
					if listener.OK {
						fmt.Fprintf(cmd.OutOrStdout(), "listener %s: ok (%s -> %s, status %d)\n",
							listener.Name,
							listener.ListenAddr,
							listener.UpstreamVersionURL,
							listener.UpstreamStatus,
						)
						continue
					}
					fmt.Fprintf(cmd.OutOrStdout(), "listener %s: %s\n", listener.Name, listener.Error)
				}
				if result.UIEnabled {
					fmt.Fprintf(cmd.OutOrStdout(), "ui: enabled on %s\n", result.UIListenAddr)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "ui: disabled")
				}
			}

			_ = logger
			if exitCode != 0 {
				return silentExit(exitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newStatsCommand(_ context.Context, opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Print aggregate usage metrics from local storage",
		Long: "Read aggregate usage metrics from the local SQLite store. " +
			"The default table output renders a compact terminal dashboard with totals, recent activity, breakdowns, sessions, and recent requests, while JSON and YAML output expose the script-friendly summary payload.",
		Args:  noArgs,
		Example: "  llamasitter stats --config llamasitter.yaml\n" +
			"  llamasitter stats --config llamasitter.yaml --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, resolved, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			store, err := app.OpenStore(cmd.Context(), cfg)
			if err != nil {
				return commandErrorf("%v", err)
			}
			defer store.Close()

			switch format {
			case outputJSON:
				summary, err := store.UsageSummary(cmd.Context(), model.RequestFilter{})
				if err != nil {
					return commandErrorf("%v", err)
				}
				if err := writeJSON(cmd.OutOrStdout(), summary); err != nil {
					return commandErrorf("%v", err)
				}
			case outputYAML:
				summary, err := store.UsageSummary(cmd.Context(), model.RequestFilter{})
				if err != nil {
					return commandErrorf("%v", err)
				}
				if err := writeYAML(cmd.OutOrStdout(), summary); err != nil {
					return commandErrorf("%v", err)
				}
			default:
				snapshot, err := loadStatsSnapshot(cmd.Context(), store, resolved, time.Now())
				if err != nil {
					return commandErrorf("%v", err)
				}
				if err := renderStatsReport(cmd.OutOrStdout(), snapshot); err != nil {
					return commandErrorf("%v", err)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newTailCommand(_ context.Context, opts *rootOptions) *cobra.Command {
	var (
		output string
		limit  int
	)

	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Show recent captured requests from local storage",
		Long: "Show the most recent requests captured in local storage. " +
			"The default table is optimized for quick terminal inspection, and JSON or YAML output can be used for scripting or deeper debugging.",
		Args:  noArgs,
		Example: "  llamasitter tail --config llamasitter.yaml -n 20\n" +
			"  llamasitter tail --config llamasitter.yaml --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			store, err := app.OpenStore(cmd.Context(), cfg)
			if err != nil {
				return commandErrorf("%v", err)
			}
			defer store.Close()

			items, err := store.ListRequests(cmd.Context(), model.RequestFilter{Limit: limit})
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				if err := writeJSON(cmd.OutOrStdout(), items); err != nil {
					return commandErrorf("%v", err)
				}
			case outputYAML:
				if err := writeYAML(cmd.OutOrStdout(), items); err != nil {
					return commandErrorf("%v", err)
				}
			default:
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "TIME\tSTATUS\tMODEL\tTOKENS\tDURATION_MS\tCLIENT\tSESSION")
				for _, item := range items {
					fmt.Fprintf(w, "%s\t%d\t%s\t%d\t%d\t%s/%s\t%s\n",
						item.StartedAt.Format(time.RFC3339),
						item.HTTPStatus,
						item.Model,
						item.TotalTokens,
						item.RequestDurationMs,
						item.ClientType,
						item.ClientInstance,
						item.SessionID,
					)
				}
				_ = w.Flush()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	cmd.Flags().IntVarP(&limit, "n", "n", 20, "number of requests to show")
	return cmd
}

func newExportCommand(_ context.Context, opts *rootOptions) *cobra.Command {
	var (
		format string
		output string
	)

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export captured requests as json or csv",
		Long: "Export captured requests from the local SQLite store in JSON or CSV form. " +
			"Use JSON for full request records and CSV when you want a compact table for spreadsheets or other analysis tools.",
		Args:  noArgs,
		Example: "  llamasitter export --config llamasitter.yaml --format json\n" +
			"  llamasitter export --config llamasitter.yaml --format csv --output requests.csv",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			store, err := app.OpenStore(cmd.Context(), cfg)
			if err != nil {
				return commandErrorf("%v", err)
			}
			defer store.Close()

			items, err := store.ListRequests(cmd.Context(), model.RequestFilter{})
			if err != nil {
				return commandErrorf("%v", err)
			}

			writer, closeFn, err := outputWriter(output, cmd.OutOrStdout())
			if err != nil {
				return commandErrorf("%v", err)
			}
			defer closeFn()

			switch format {
			case "json":
				enc := json.NewEncoder(writer)
				enc.SetIndent("", "  ")
				if err := enc.Encode(items); err != nil {
					return commandErrorf("%v", err)
				}
			case "csv":
				cw := csv.NewWriter(writer)
				_ = cw.Write([]string{"request_id", "started_at", "model", "http_status", "total_tokens", "request_duration_ms"})
				for _, item := range items {
					_ = cw.Write([]string{
						item.RequestID,
						item.StartedAt.Format(time.RFC3339),
						item.Model,
						strconv.Itoa(item.HTTPStatus),
						strconv.FormatInt(item.TotalTokens, 10),
						strconv.FormatInt(item.RequestDurationMs, 10),
					})
				}
				cw.Flush()
				if err := cw.Error(); err != nil {
					return commandErrorf("%v", err)
				}
			default:
				return usageErrorf("unsupported format %q (expected json or csv)", format)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "json", "export format: json or csv")
	cmd.Flags().StringVar(&output, "output", "-", "output path or - for stdout")
	return cmd
}

func outputWriter(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return stdout, func() {}, nil
	}

	file, err := os.Create(path)
	if err != nil {
		return nil, nil, err
	}
	return file, func() { _ = file.Close() }, nil
}

func versionEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	parsed.Path = path.Join(parsed.Path, "/api/version")
	return parsed.String(), nil
}
