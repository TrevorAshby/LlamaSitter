package cli

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/configedit"
)

func newConfigCommand(_ context.Context, logger *slog.Logger, opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect, validate, and safely mutate LlamaSitter config files",
		Long: "Inspect the current config, create a new one, validate it, and update specific sections without hand-editing YAML. " +
			"Most mutating config commands support --dry-run so you can preview the exact file contents before writing them.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newConfigPathCommand(opts),
		newConfigInitCommand(opts),
		newConfigViewCommand(opts),
		newConfigValidateCommand(opts),
		newConfigListenerCommand(opts),
		newConfigUICommand(opts),
		newConfigStorageCommand(opts),
		newConfigPrivacyCommand(opts),
	)

	_ = logger
	return cmd
}

func newConfigPathCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the resolved path of the target config file",
		Long: "Print the exact config file path that the current command invocation will use after applying the --config flag and default path rules.",
		Args:  noArgs,
		Example: "  llamasitter config path\n" +
			"  llamasitter config path --config /tmp/llamasitter.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveConfigPath(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), resolved)
			return nil
		},
	}
}

func newConfigInitCommand(opts *rootOptions) *cobra.Command {
	var (
		force  bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a default config file at the selected path",
		Long: "Create a new default config file at the selected path. " +
			"Use --dry-run to preview the generated YAML or --force to overwrite an existing file deliberately.",
		Args:  noArgs,
		Example: "  llamasitter config init\n" +
			"  llamasitter config init --config /tmp/llamasitter.yaml --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := resolveConfigPath(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if fileExists(resolved) && !force {
				return commandErrorf("config already exists at %s (use --force to overwrite)", resolved)
			}

			doc := configedit.NewDefault()
			if _, err := doc.Config(); err != nil {
				return commandErrorf("validate config: %v", err)
			}

			if dryRun {
				raw, err := doc.Bytes()
				if err != nil {
					return commandErrorf("%v", err)
				}
				return commandErrorfFrom(writeRawYAML(cmd.OutOrStdout(), raw))
			}

			if err := doc.WriteAtomic(resolved); err != nil {
				return commandErrorf("write config: %v", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Created config: %s\n", resolved)
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", restartHint(resolved))
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing config file")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the generated config without writing it")
	return cmd
}

func newConfigViewCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "view",
		Short: "Render the current config as yaml, json, or a summary table",
		Long: "Load the selected config file and render it as YAML, JSON, or a compact summary table. " +
			"This is the fastest way to confirm what LlamaSitter will actually run with.",
		Args:  noArgs,
		Example: "  llamasitter config view\n" +
			"  llamasitter config view --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			cfg, err := doc.Config()
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg))
			case outputYAML:
				raw, err := doc.Bytes()
				if err != nil {
					return commandErrorf("%v", err)
				}
				return commandErrorfFrom(writeRawYAML(cmd.OutOrStdout(), raw))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "config\t%s\n", resolved)
				fmt.Fprintf(cmd.OutOrStdout(), "listeners\t%d\n", len(cfg.Listeners))
				fmt.Fprintf(cmd.OutOrStdout(), "storage\t%s\n", cfg.Storage.SQLitePath)
				if cfg.UI.Enabled {
					fmt.Fprintf(cmd.OutOrStdout(), "ui\tenabled on %s\n", cfg.UI.ListenAddr)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "ui\tdisabled")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "yaml", "output format: table, json, or yaml")
	return cmd
}

func newConfigValidateCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the selected config file without contacting upstreams",
		Long: "Parse and validate the selected config file without opening storage or contacting upstream Ollama instances. " +
			"Use this when you only want a structural and semantic config check.",
		Args:  noArgs,
		Example: "  llamasitter config validate\n" +
			"  llamasitter config validate --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, resolved, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), cfg))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "config: valid (%s)\n", resolved)
				fmt.Fprintf(cmd.OutOrStdout(), "listeners: %d\n", len(cfg.Listeners))
				fmt.Fprintf(cmd.OutOrStdout(), "storage: %s\n", cfg.Storage.SQLitePath)
				if cfg.UI.Enabled {
					fmt.Fprintf(cmd.OutOrStdout(), "ui: enabled on %s\n", cfg.UI.ListenAddr)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "ui: disabled")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigListenerCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listener",
		Short: "List, inspect, add, update, tag, and remove listeners",
		Long: "Manage the configured proxy listeners. " +
			"Each listener defines a local bind address, an upstream Ollama base URL, and optional default attribution tags that are applied when requests do not provide explicit X-LlamaSitter-* headers. " +
			"Common identity keys are client_type, client_instance, agent_name, session_id, run_id, and workspace.",
		Args:  noArgs,
		Example: "  llamasitter config listener list\n" +
			"  llamasitter config listener show default\n" +
			"  llamasitter config listener set-tag default client_type=desktop-app",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newConfigListenerListCommand(opts),
		newConfigListenerShowCommand(opts),
		newConfigListenerAddCommand(opts),
		newConfigListenerUpdateCommand(opts),
		newConfigListenerSetTagCommand(opts),
		newConfigListenerUnsetTagCommand(opts),
		newConfigListenerRemoveCommand(opts),
	)

	return cmd
}

func newConfigListenerListCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configured listeners",
		Long: "List all listeners defined in the selected config file, including their local bind addresses, upstream URLs, and default tags.",
		Args:  noArgs,
		Example: "  llamasitter config listener list\n" +
			"  llamasitter config listener list --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}
			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg.Listeners))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), cfg.Listeners))
			default:
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tLISTEN_ADDR\tUPSTREAM_URL\tDEFAULT_TAGS")
				for _, listener := range cfg.Listeners {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
						listener.Name,
						listener.ListenAddr,
						listener.UpstreamURL,
						formatTags(listener.DefaultTags),
					)
				}
				_ = w.Flush()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigListenerShowCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show one configured listener",
		Long: "Show the full configuration for a single listener identified by name.",
		Args:  exactArgs(1),
		Example: "  llamasitter config listener show default\n" +
			"  llamasitter config listener show default --output yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}
			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			listener, found := findListenerByName(cfg.Listeners, args[0])
			if !found {
				return commandErrorf("listener %q not found", args[0])
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), listener))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), listener))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "name\t%s\n", listener.Name)
				fmt.Fprintf(cmd.OutOrStdout(), "listen_addr\t%s\n", listener.ListenAddr)
				fmt.Fprintf(cmd.OutOrStdout(), "upstream_url\t%s\n", listener.UpstreamURL)
				fmt.Fprintf(cmd.OutOrStdout(), "default_tags\t%s\n", formatTags(listener.DefaultTags))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigListenerAddCommand(opts *rootOptions) *cobra.Command {
	var (
		name        string
		listenAddr  string
		upstreamURL string
		tags        []string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a new listener to the selected config file",
		Long: "Add a new listener to the selected config file. " +
			"A listener needs a name, a local listen address, and an upstream Ollama base URL, and it can optionally define default attribution tags. " +
			"Stable fields like client_type, client_instance, and workspace are often good listener defaults.",
		Args:  noArgs,
		Example: "  llamasitter config listener add --name openwebui --listen-addr 127.0.0.1:11436 --upstream-url http://127.0.0.1:11434\n" +
			"  llamasitter config listener add --name openwebui --listen-addr 0.0.0.0:11436 --upstream-url http://127.0.0.1:11434 --tag client_type=openwebui --tag client_instance=docker\n" +
			"  llamasitter config listener add --name agentbox --listen-addr 127.0.0.1:11437 --upstream-url http://127.0.0.1:11434 --tag workspace=/srv/agentbox",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(listenAddr) == "" || strings.TrimSpace(upstreamURL) == "" {
				_ = cmd.Usage()
				return usageErrorf("--name, --listen-addr, and --upstream-url are required")
			}

			doc, resolved, err := loadOrCreateConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			tagMap, err := parseTagAssignments(tags)
			if err != nil {
				return usageErrorf("%v", err)
			}

			if err := doc.AddListener(config.ListenerConfig{
				Name:        name,
				ListenAddr:  listenAddr,
				UpstreamURL: upstreamURL,
				DefaultTags: tagMap,
			}); err != nil {
				return commandErrorf("%v", err)
			}

			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Added listener %q.", name))
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "listener name")
	cmd.Flags().StringVar(&listenAddr, "listen-addr", "", "listener bind address, for example 127.0.0.1:11435")
	cmd.Flags().StringVar(&upstreamURL, "upstream-url", "", "upstream Ollama base URL")
	cmd.Flags().StringSliceVar(&tags, "tag", nil, "default tag assignment in KEY=VALUE form (repeatable); common keys: client_type, client_instance, agent_name, session_id, run_id, workspace")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigListenerUpdateCommand(opts *rootOptions) *cobra.Command {
	var (
		rename      string
		listenAddr  string
		upstreamURL string
		dryRun      bool
	)

	cmd := &cobra.Command{
		Use:   "update NAME",
		Short: "Update listener name, bind address, or upstream URL",
		Long: "Update an existing listener's name, local bind address, or upstream Ollama URL without rewriting the rest of the config file.",
		Args:  exactArgs(1),
		Example: "  llamasitter config listener update default --listen-addr 127.0.0.1:11439\n" +
			"  llamasitter config listener update openwebui --rename ui-docker --upstream-url http://127.0.0.1:11434",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rename == "" && listenAddr == "" && upstreamURL == "" {
				_ = cmd.Usage()
				return usageErrorf("at least one of --rename, --listen-addr, or --upstream-url is required")
			}

			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			update := configedit.ListenerUpdate{}
			if rename != "" {
				update.Rename = &rename
			}
			if listenAddr != "" {
				update.ListenAddr = &listenAddr
			}
			if upstreamURL != "" {
				update.UpstreamURL = &upstreamURL
			}

			if err := doc.UpdateListener(args[0], update); err != nil {
				return commandErrorf("%v", err)
			}

			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Updated listener %q.", args[0]))
		},
	}

	cmd.Flags().StringVar(&rename, "rename", "", "rename the listener")
	cmd.Flags().StringVar(&listenAddr, "listen-addr", "", "set a new bind address")
	cmd.Flags().StringVar(&upstreamURL, "upstream-url", "", "set a new upstream URL")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigListenerSetTagCommand(opts *rootOptions) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "set-tag NAME KEY=VALUE",
		Short: "Set or replace one default tag on a listener",
		Long: "Set or replace one default attribution tag on a listener. " +
			"These tags are used when incoming requests do not send explicit X-LlamaSitter-* headers for the same fields. " +
			"Good listener defaults are usually stable fields like client_type, client_instance, and workspace; session_id and run_id are usually better sent per request.",
		Args:  exactArgs(2),
		Example: "  llamasitter config listener set-tag openwebui client_type=openwebui\n" +
			"  llamasitter config listener set-tag openwebui client_instance=docker\n" +
			"  llamasitter config listener set-tag openwebui workspace=/Users/me/project",
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value, err := splitTag(args[1])
			if err != nil {
				return usageErrorf("%v", err)
			}

			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.SetListenerTag(args[0], key, value); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Updated tag %q on listener %q.", key, args[0]))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigListenerUnsetTagCommand(opts *rootOptions) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "unset-tag NAME KEY",
		Short:   "Remove one default tag from a listener",
		Long:    "Remove one default attribution tag from a listener. This only removes the listener default and does not affect explicit X-LlamaSitter-* headers sent by callers.",
		Args:    exactArgs(2),
		Example: "  llamasitter config listener unset-tag openwebui client_type",
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.UnsetListenerTag(args[0], args[1]); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Removed tag %q from listener %q.", args[1], args[0]))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigListenerRemoveCommand(opts *rootOptions) *cobra.Command {
	var (
		yes    bool
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove one listener from the selected config file",
		Long: "Remove one listener from the selected config file. " +
			"Because this is destructive, the command requires --yes unless you are only previewing the result with --dry-run.",
		Args:  exactArgs(1),
		Example: "  llamasitter config listener remove openwebui --yes\n" +
			"  llamasitter config listener remove openwebui --yes --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes && !dryRun {
				_ = cmd.Usage()
				return usageErrorf("--yes is required to remove a listener")
			}

			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.RemoveListener(args[0]); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Removed listener %q.", args[0]))
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "confirm destructive removal")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigUICommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Inspect and update dashboard UI settings",
		Long: "Inspect or change the embedded dashboard UI settings, including whether the UI is enabled and which local address it listens on.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newConfigUIShowCommand(opts),
		newConfigUIEnableCommand(opts, true),
		newConfigUIEnableCommand(opts, false),
		newConfigUISetListenAddrCommand(opts),
	)

	return cmd
}

func newConfigUIShowCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show UI enablement and listen address",
		Long: "Show whether the embedded dashboard UI is enabled and which local address it is configured to use.",
		Args:  noArgs,
		Example: "  llamasitter config ui show\n" +
			"  llamasitter config ui show --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg.UI))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), cfg.UI))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "enabled\t%s\n", strconv.FormatBool(cfg.UI.Enabled))
				fmt.Fprintf(cmd.OutOrStdout(), "listen_addr\t%s\n", cfg.UI.ListenAddr)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigUIEnableCommand(opts *rootOptions, enabled bool) *cobra.Command {
	var dryRun bool
	use := "enable"
	short := "Enable the dashboard UI listener"
	success := "Enabled the dashboard UI."
	if !enabled {
		use = "disable"
		short = "Disable the dashboard UI listener"
		success = "Disabled the dashboard UI."
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long: "Update whether the embedded dashboard UI listener is enabled in the selected config file.",
		Args:  noArgs,
		Example: "  llamasitter config ui " + use + "\n" +
			"  llamasitter config ui " + use + " --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.SetUIEnabled(enabled); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, success)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigUISetListenAddrCommand(opts *rootOptions) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "set-listen-addr HOST:PORT",
		Short:   "Set the UI listener bind address",
		Long:    "Set the local bind address for the embedded dashboard UI listener.",
		Args:    exactArgs(1),
		Example: "  llamasitter config ui set-listen-addr 127.0.0.1:11439",
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.SetUIListenAddr(args[0]); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Updated UI listen address to %s.", args[0]))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigStorageCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Inspect and update storage settings",
		Long: "Inspect or update storage settings such as the SQLite database path used for request persistence.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newConfigStorageShowCommand(opts),
		newConfigStorageSetSQLitePathCommand(opts),
	)

	return cmd
}

func newConfigStorageShowCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show storage settings",
		Long: "Show the currently configured storage settings, including the resolved SQLite database path.",
		Args:  noArgs,
		Example: "  llamasitter config storage show\n" +
			"  llamasitter config storage show --output yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg.Storage))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), cfg.Storage))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "sqlite_path\t%s\n", cfg.Storage.SQLitePath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigStorageSetSQLitePathCommand(opts *rootOptions) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "set-sqlite-path PATH",
		Short:   "Set the SQLite database path",
		Long:    "Update the SQLite database path in the selected config file.",
		Args:    exactArgs(1),
		Example: "  llamasitter config storage set-sqlite-path ~/.llamasitter/custom.db",
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.SetStorageSQLitePath(args[0]); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Updated SQLite path to %s.", args[0]))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigPrivacyCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "privacy",
		Short: "Inspect and update privacy and redaction settings",
		Long: "Inspect or update privacy-related storage behavior such as whether bodies are persisted and which headers or JSON fields are redacted.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newConfigPrivacyShowCommand(opts),
		newConfigPrivacySetPersistBodiesCommand(opts),
		newConfigPrivacyAddHeaderCommand(opts),
		newConfigPrivacyRemoveHeaderCommand(opts),
		newConfigPrivacyAddFieldCommand(opts),
		newConfigPrivacyRemoveFieldCommand(opts),
	)

	return cmd
}

func newConfigPrivacyShowCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show privacy and redaction settings",
		Long: "Show whether prompt and response bodies are persisted and which headers or JSON fields are currently configured for redaction.",
		Args:  noArgs,
		Example: "  llamasitter config privacy show\n" +
			"  llamasitter config privacy show --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			cfg, _, err := loadConfig(cmd.Context(), opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), cfg.Privacy))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), cfg.Privacy))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "persist_bodies\t%s\n", strconv.FormatBool(cfg.Privacy.PersistBodies))
				fmt.Fprintf(cmd.OutOrStdout(), "redact_headers\t%s\n", strings.Join(cfg.Privacy.RedactHeaders, ", "))
				fmt.Fprintf(cmd.OutOrStdout(), "redact_json_fields\t%s\n", strings.Join(cfg.Privacy.RedactJSONFields, ", "))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newConfigPrivacySetPersistBodiesCommand(opts *rootOptions) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "set-persist-bodies true|false",
		Short: "Enable or disable prompt/response body persistence",
		Long: "Enable or disable persistence of prompt and response bodies in the selected config file. " +
			"Turning this off keeps LlamaSitter focused on metadata, counters, and attribution rather than body content.",
		Args:  exactArgs(1),
		Example: "  llamasitter config privacy set-persist-bodies false\n" +
			"  llamasitter config privacy set-persist-bodies true --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			enabled, err := strconv.ParseBool(args[0])
			if err != nil {
				return usageErrorf("persist_bodies expects true or false")
			}

			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := doc.SetPersistBodies(enabled); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf("Updated persist_bodies to %t.", enabled))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func newConfigPrivacyAddHeaderCommand(opts *rootOptions) *cobra.Command {
	return newPrivacyListMutationCommand(
		opts,
		"add-redact-header NAME",
		"Add one header name to redact_headers",
		"  llamasitter config privacy add-redact-header authorization",
		func(doc *configedit.Document, value string) error { return doc.AddRedactHeader(value) },
		"Added redact header %q.",
	)
}

func newConfigPrivacyRemoveHeaderCommand(opts *rootOptions) *cobra.Command {
	return newPrivacyListMutationCommand(
		opts,
		"remove-redact-header NAME",
		"Remove one header name from redact_headers",
		"  llamasitter config privacy remove-redact-header authorization",
		func(doc *configedit.Document, value string) error { return doc.RemoveRedactHeader(value) },
		"Removed redact header %q.",
	)
}

func newConfigPrivacyAddFieldCommand(opts *rootOptions) *cobra.Command {
	return newPrivacyListMutationCommand(
		opts,
		"add-redact-json-field NAME",
		"Add one field name to redact_json_fields",
		"  llamasitter config privacy add-redact-json-field messages",
		func(doc *configedit.Document, value string) error { return doc.AddRedactJSONField(value) },
		"Added redact JSON field %q.",
	)
}

func newConfigPrivacyRemoveFieldCommand(opts *rootOptions) *cobra.Command {
	return newPrivacyListMutationCommand(
		opts,
		"remove-redact-json-field NAME",
		"Remove one field name from redact_json_fields",
		"  llamasitter config privacy remove-redact-json-field messages",
		func(doc *configedit.Document, value string) error { return doc.RemoveRedactJSONField(value) },
		"Removed redact JSON field %q.",
	)
}

func newPrivacyListMutationCommand(opts *rootOptions, use, short, example string, apply func(*configedit.Document, string) error, successFormat string) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Args:    exactArgs(1),
		Example: example,
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, resolved, err := loadConfigDocument(opts.ConfigPath)
			if err != nil {
				return commandErrorf("%v", err)
			}
			if err := apply(doc, args[0]); err != nil {
				return commandErrorf("%v", err)
			}
			return persistMutation(cmd, doc, resolved, dryRun, fmt.Sprintf(successFormat, args[0]))
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the updated config without writing it")
	return cmd
}

func loadOrCreateConfigDocument(path string) (*configedit.Document, string, error) {
	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, "", err
	}
	if !fileExists(resolved) {
		return configedit.NewDefault(), resolved, nil
	}
	return loadConfigDocument(path)
}

func persistMutation(cmd *cobra.Command, doc *configedit.Document, path string, dryRun bool, success string) error {
	if _, err := doc.Config(); err != nil {
		return commandErrorf("validate config: %v", err)
	}

	raw, err := doc.Bytes()
	if err != nil {
		return commandErrorf("%v", err)
	}

	if dryRun {
		return commandErrorfFrom(writeRawYAML(cmd.OutOrStdout(), raw))
	}

	if err := doc.WriteAtomic(path); err != nil {
		return commandErrorf("write config: %v", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), success)
	fmt.Fprintf(cmd.OutOrStdout(), "Updated config: %s\n", path)
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", restartHint(path))
	return nil
}

func findListenerByName(listeners []config.ListenerConfig, name string) (config.ListenerConfig, bool) {
	for _, listener := range listeners {
		if listener.Name == name {
			return listener, true
		}
	}
	return config.ListenerConfig{}, false
}

func parseTagAssignments(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(values))
	for _, value := range values {
		key, parsed, err := splitTag(value)
		if err != nil {
			return nil, err
		}
		out[key] = parsed
	}
	return out, nil
}

func splitTag(value string) (string, string, error) {
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("expected KEY=VALUE, got %q", value)
	}
	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", fmt.Errorf("tag key must not be empty")
	}
	return key, parts[1], nil
}

func formatTags(tags map[string]string) string {
	if len(tags) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(tags))
	for _, key := range sortedKeys(tags) {
		parts = append(parts, fmt.Sprintf("%s=%s", key, tags[key]))
	}
	return strings.Join(parts, ", ")
}

func commandErrorfFrom(err error) error {
	if err == nil {
		return nil
	}
	return commandErrorf("%v", err)
}

func init() {
	cobra.EnableCommandSorting = true
}
