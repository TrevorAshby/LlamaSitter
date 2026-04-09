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

			maybeLaunchDesktopCompanion(runCtx, opts.ConfigPath, logger)

			if err := app.Run(runCtx, cfg, logger); err != nil {
				return commandErrorf("%v", err)
			}
			return nil
		},
	}
}

func maybeLaunchDesktopCompanion(ctx context.Context, configPath string, logger *slog.Logger) {
	if runtime.GOOS != "darwin" {
		return
	}
	if os.Getenv("LLAMASITTER_DESKTOP_MANAGED") != "" || os.Getenv("LLAMASITTER_NO_DESKTOP_AUTO_LAUNCH") == "1" {
		return
	}

	resolvedConfigPath, err := resolveConfigPath(configPath)
	if err != nil {
		if logger != nil {
			logger.Warn("desktop companion auto-launch skipped", "reason", err.Error())
		}
		return
	}

	companionBundlePath := firstExistingPath(desktopCompanionBundleCandidates())
	if companionBundlePath == "" {
		if logger != nil {
			logger.Info("desktop companion not found; continuing without menu icon", "config", resolvedConfigPath)
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

func firstExistingPath(paths []string) string {
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
		Args:  noArgs,
		Example: "  llamasitter stats --config llamasitter.yaml\n" +
			"  llamasitter stats --config llamasitter.yaml --output json",
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

			summary, err := store.UsageSummary(cmd.Context(), model.RequestFilter{})
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				if err := writeJSON(cmd.OutOrStdout(), summary); err != nil {
					return commandErrorf("%v", err)
				}
			case outputYAML:
				if err := writeYAML(cmd.OutOrStdout(), summary); err != nil {
					return commandErrorf("%v", err)
				}
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "requests\t%d\n", summary.RequestCount)
				fmt.Fprintf(cmd.OutOrStdout(), "successful\t%d\n", summary.SuccessCount)
				fmt.Fprintf(cmd.OutOrStdout(), "aborted\t%d\n", summary.AbortedCount)
				fmt.Fprintf(cmd.OutOrStdout(), "active_sessions\t%d\n", summary.ActiveSessionCount)
				fmt.Fprintf(cmd.OutOrStdout(), "prompt_tokens\t%d\n", summary.PromptTokens)
				fmt.Fprintf(cmd.OutOrStdout(), "output_tokens\t%d\n", summary.OutputTokens)
				fmt.Fprintf(cmd.OutOrStdout(), "total_tokens\t%d\n", summary.TotalTokens)
				fmt.Fprintf(cmd.OutOrStdout(), "avg_duration_ms\t%.2f\n", summary.AvgRequestDurationMs)
				if len(summary.ByModel) > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "\nby_model:")
					for _, row := range summary.ByModel {
						fmt.Fprintf(cmd.OutOrStdout(), "- %s: %d requests, %d total tokens\n", row.Key, row.RequestCount, row.TotalTokens)
					}
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
