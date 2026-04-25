package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/desktop"
)

func desktopPaths() (desktop.Paths, error) {
	return desktop.ManagedPaths()
}

func newDesktopCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Inspect desktop app-managed paths and helpers",
		Long: "Print the config, database, log, runtime, and autostart details managed by the desktop shells " +
			"so the CLI and native desktop wrappers can target the same files intentionally.",
		Args: noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the desktop-managed config path",
		Long:  "Inspect the config path used by the desktop-managed runtime.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	configCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the desktop-managed config path", func(paths desktop.Paths) string {
		return paths.Config
	}))

	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Inspect the desktop-managed database path",
		Long:  "Inspect the SQLite database path used by the desktop-managed runtime.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	dbCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the desktop-managed SQLite database path", func(paths desktop.Paths) string {
		return paths.DB
	}))

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Inspect the desktop-managed logs path",
		Long:  "Inspect the logs directory used by the desktop-managed runtime.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	logsCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the desktop-managed logs directory", func(paths desktop.Paths) string {
		return paths.Logs
	}))

	cmd.AddCommand(
		configCmd,
		dbCmd,
		logsCmd,
		newDesktopRuntimeCommand(opts),
		newDesktopAutostartCommand(opts),
	)
	return cmd
}

func newDesktopPathLeafCommand(use, short string, selectPath func(desktop.Paths) string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Long:  short,
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := desktopPaths()
			if err != nil {
				return commandErrorf("%v", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), selectPath(paths))
			return nil
		},
	}
}

func newDesktopRuntimeCommand(opts *rootOptions) *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:    "runtime",
		Short:  "Render the resolved desktop runtime as JSON",
		Long:   "Render the resolved desktop runtime contract, including managed paths and derived URLs, as JSON for the native desktop shells.",
		Args:   noArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeInfo, err := resolveDesktopRuntime(cmd, opts)
			if err != nil {
				return commandErrorf("%v", err)
			}

			switch output {
			case "json", "":
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), runtimeInfo))
			case "yaml":
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), runtimeInfo))
			default:
				return usageErrorf("unsupported output format %q (expected json or yaml)", output)
			}
		},
	}

	cmd.Flags().StringVar(&output, "output", "json", "output format: json or yaml")
	return cmd
}

func newDesktopAutostartCommand(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "autostart",
		Short: "Inspect and manage Linux desktop autostart",
		Long:  "Inspect and manage the Linux desktop autostart entry used to launch the LlamaSitter tray agent on login.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.AddCommand(
		newDesktopAutostartStatusCommand(),
		newDesktopAutostartEnableCommand(opts),
		newDesktopAutostartDisableCommand(),
	)
	return cmd
}

func newDesktopAutostartStatusCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show whether Linux desktop autostart is enabled",
		Long:  "Show whether Linux desktop autostart is enabled and where the desktop entry file lives.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := desktop.AutostartState()
			if err != nil {
				return commandErrorf("%v", err)
			}
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}
			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), status))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), status))
			case outputTable:
				fmt.Fprintf(cmd.OutOrStdout(), "enabled\t%t\n", status.Enabled)
				fmt.Fprintf(cmd.OutOrStdout(), "path\t%s\n", status.Path)
				return nil
			default:
				return usageErrorf("unsupported output format %q (expected table, json, or yaml)", output)
			}
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}

func newDesktopAutostartEnableCommand(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable Linux desktop autostart for the tray agent",
		Long:  "Write a Linux desktop autostart entry that starts the tray agent in attach-only mode when you log in.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runtimeInfo, err := resolveDesktopRuntime(cmd, opts)
			if err != nil {
				return commandErrorf("%v", err)
			}
			status, err := desktop.EnableAutostart(runtimeInfo.ConfigPath, "")
			if err != nil {
				return commandErrorf("%v", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Enabled desktop autostart: %s\n", status.Path)
			return nil
		},
	}
}

func newDesktopAutostartDisableCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable Linux desktop autostart for the tray agent",
		Long:  "Remove the Linux desktop autostart entry for the tray agent if it exists.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := desktop.DisableAutostart()
			if err != nil {
				return commandErrorf("%v", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Disabled desktop autostart: %s\n", status.Path)
			return nil
		},
	}
}

func resolveDesktopRuntime(cmd *cobra.Command, opts *rootOptions) (desktop.Runtime, error) {
	configPath, err := desktop.ResolveConfigPath(opts.ConfigPath, configFlagChanged(cmd))
	if err != nil {
		return desktop.Runtime{}, err
	}
	return desktop.ResolveRuntime(configPath, desktop.AttachOnlyFromEnv(), "")
}

func configFlagChanged(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if flag := cmd.Flags().Lookup("config"); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.InheritedFlags().Lookup("config"); flag != nil && flag.Changed {
		return true
	}
	if flag := cmd.Root().PersistentFlags().Lookup("config"); flag != nil && flag.Changed {
		return true
	}
	return false
}
