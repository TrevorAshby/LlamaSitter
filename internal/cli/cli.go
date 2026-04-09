package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/trevorashby/llamasitter/internal/app"
	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/model"
)

func Run(ctx context.Context, args []string, logger *slog.Logger) int {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "serve":
		return runServe(ctx, args[1:], logger)
	case "doctor":
		return runDoctor(ctx, args[1:], logger)
	case "stats":
		return runStats(ctx, args[1:])
	case "tail":
		return runTail(ctx, args[1:])
	case "export":
		return runExport(ctx, args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func runServe(ctx context.Context, args []string, logger *slog.Logger) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultPath, "path to config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	runCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()

	if err := app.Run(runCtx, cfg, logger); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runDoctor(ctx context.Context, args []string, logger *slog.Logger) int {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultPath, "path to config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "config: ok (%s)\n", *configPath)
	fmt.Fprintf(os.Stdout, "storage: %s\n", cfg.Storage.SQLitePath)

	store, err := app.OpenStore(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "storage:", err)
		return 1
	}
	defer store.Close()
	fmt.Fprintln(os.Stdout, "storage: ok")

	client := &http.Client{Timeout: 3 * time.Second}
	exitCode := 0
	for _, listener := range cfg.Listeners {
		versionURL, err := versionEndpoint(listener.UpstreamURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "listener %s: invalid upstream url: %v\n", listener.Name, err)
			exitCode = 1
			continue
		}

		resp, err := client.Get(versionURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "listener %s: upstream unreachable: %v\n", listener.Name, err)
			exitCode = 1
			continue
		}
		_ = resp.Body.Close()
		fmt.Fprintf(os.Stdout, "listener %s: ok (%s -> %s, status %d)\n", listener.Name, listener.ListenAddr, versionURL, resp.StatusCode)
	}

	if cfg.UI.Enabled {
		fmt.Fprintf(os.Stdout, "ui: enabled on %s\n", cfg.UI.ListenAddr)
	} else {
		fmt.Fprintln(os.Stdout, "ui: disabled")
	}

	_ = logger
	return exitCode
}

func runStats(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("stats", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultPath, "path to config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	store, err := app.OpenStore(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer store.Close()

	summary, err := store.UsageSummary(ctx, model.RequestFilter{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "requests\t%d\n", summary.RequestCount)
	fmt.Fprintf(os.Stdout, "successful\t%d\n", summary.SuccessCount)
	fmt.Fprintf(os.Stdout, "aborted\t%d\n", summary.AbortedCount)
	fmt.Fprintf(os.Stdout, "prompt_tokens\t%d\n", summary.PromptTokens)
	fmt.Fprintf(os.Stdout, "output_tokens\t%d\n", summary.OutputTokens)
	fmt.Fprintf(os.Stdout, "total_tokens\t%d\n", summary.TotalTokens)
	fmt.Fprintf(os.Stdout, "avg_duration_ms\t%.2f\n", summary.AvgRequestDurationMs)

	if len(summary.ByModel) > 0 {
		fmt.Fprintln(os.Stdout, "\nby_model:")
		for _, row := range summary.ByModel {
			fmt.Fprintf(os.Stdout, "- %s: %d requests, %d total tokens\n", row.Key, row.RequestCount, row.TotalTokens)
		}
	}

	return 0
}

func runTail(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("tail", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultPath, "path to config file")
	limit := flags.Int("n", 20, "number of requests to show")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	store, err := app.OpenStore(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer store.Close()

	items, err := store.ListRequests(ctx, model.RequestFilter{Limit: *limit})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
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
	return 0
}

func runExport(ctx context.Context, args []string) int {
	flags := flag.NewFlagSet("export", flag.ContinueOnError)
	configPath := flags.String("config", config.DefaultPath, "path to config file")
	format := flags.String("format", "json", "json or csv")
	output := flags.String("output", "-", "output path or - for stdout")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	store, err := app.OpenStore(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer store.Close()

	items, err := store.ListRequests(ctx, model.RequestFilter{})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	writer, closeFn, err := outputWriter(*output)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer closeFn()

	switch *format {
	case "json":
		enc := json.NewEncoder(writer)
		enc.SetIndent("", "  ")
		if err := enc.Encode(items); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
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
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
	default:
		fmt.Fprintf(os.Stderr, "unsupported format %q\n", *format)
		return 2
	}

	return 0
}

func outputWriter(path string) (io.Writer, func(), error) {
	if path == "" || path == "-" {
		return os.Stdout, func() {}, nil
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

func printUsage(w io.Writer) {
	_, _ = io.WriteString(w, `LlamaSitter

Usage:
  llamasitter serve  -config llamasitter.yaml
  llamasitter doctor -config llamasitter.yaml
  llamasitter stats  -config llamasitter.yaml
  llamasitter tail   -config llamasitter.yaml -n 20
  llamasitter export -config llamasitter.yaml -format json
`)
}
