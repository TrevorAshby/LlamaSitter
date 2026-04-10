package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

type desktopPathSet struct {
	Config string
	DB     string
	Logs   string
}

func desktopPaths() (desktopPathSet, error) {
	if runtime.GOOS != "darwin" {
		return desktopPathSet{}, fmt.Errorf("desktop helpers are only available on macOS")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return desktopPathSet{}, err
	}

	appSupport := filepath.Join(home, "Library", "Application Support", "LlamaSitter")
	return desktopPathSet{
		Config: filepath.Join(appSupport, "llamasitter.yaml"),
		DB:     filepath.Join(appSupport, "llamasitter.db"),
		Logs:   filepath.Join(home, "Library", "Logs", "LlamaSitter"),
	}, nil
}

func newDesktopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desktop",
		Short: "Inspect macOS desktop app-managed paths",
		Long: "Print the config, database, and log paths managed by the macOS desktop app so the CLI can target the same files intentionally.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect the app-managed desktop config path",
		Long: "Inspect the config path used by the macOS desktop app.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	configCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the app-managed config path", func(paths desktopPathSet) string {
		return paths.Config
	}))

	dbCmd := &cobra.Command{
		Use:   "db",
		Short: "Inspect the app-managed desktop database path",
		Long: "Inspect the SQLite database path used by the macOS desktop app.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	dbCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the app-managed SQLite database path", func(paths desktopPathSet) string {
		return paths.DB
	}))

	logsCmd := &cobra.Command{
		Use:   "logs",
		Short: "Inspect the app-managed desktop logs path",
		Long: "Inspect the logs directory used by the macOS desktop app.",
		Args:  noArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}
	logsCmd.AddCommand(newDesktopPathLeafCommand("path", "Print the app-managed logs directory", func(paths desktopPathSet) string {
		return paths.Logs
	}))

	cmd.AddCommand(configCmd, dbCmd, logsCmd)
	return cmd
}

func newDesktopPathLeafCommand(use, short string, selectPath func(desktopPathSet) string) *cobra.Command {
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
