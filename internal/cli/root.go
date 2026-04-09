package cli

import (
	"context"
	"io"
	"log/slog"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/config"
)

type rootOptions struct {
	ConfigPath string
}

func newRootCommand(ctx context.Context, logger *slog.Logger, stdout, stderr io.Writer) *cobra.Command {
	opts := &rootOptions{
		ConfigPath: config.DefaultPath,
	}

	cmd := &cobra.Command{
		Use:   "llamasitter",
		Short: "Observe and manage local Ollama traffic through LlamaSitter",
		Long: "LlamaSitter is a local-first observability proxy for Ollama. " +
			"It captures usage, timing, model activity, and attribution while remaining transparent to callers.",
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return silentExit(2)
		},
	}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		_ = cmd.Usage()
		return usageErrorf("%s", err)
	})
	cmd.PersistentFlags().StringVarP(&opts.ConfigPath, "config", "c", config.DefaultPath, "path to config file")

	cmd.AddCommand(
		newServeCommand(ctx, logger, opts),
		newDoctorCommand(ctx, logger, opts),
		newStatsCommand(ctx, opts),
		newTailCommand(ctx, opts),
		newExportCommand(ctx, opts),
		newVersionCommand(),
		newCompletionCommand(),
		newConfigCommand(ctx, logger, opts),
		newDesktopCommand(),
	)

	disableAutoGenTag(cmd)
	return cmd
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, child := range cmd.Commands() {
		disableAutoGenTag(child)
	}
}
