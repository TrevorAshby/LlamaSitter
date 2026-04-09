package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print LlamaSitter build version metadata",
		Args:  noArgs,
		Example: "  llamasitter version\n" +
			"  llamasitter version --output json",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := parseInspectOutput(output)
			if err != nil {
				return usageErrorf("%v", err)
			}

			info := buildinfo.Get()
			switch format {
			case outputJSON:
				return commandErrorfFrom(writeJSON(cmd.OutOrStdout(), info))
			case outputYAML:
				return commandErrorfFrom(writeYAML(cmd.OutOrStdout(), info))
			default:
				fmt.Fprintf(cmd.OutOrStdout(), "version\t%s\n", info.Version)
				fmt.Fprintf(cmd.OutOrStdout(), "commit\t%s\n", info.Commit)
				fmt.Fprintf(cmd.OutOrStdout(), "date\t%s\n", info.Date)
				return nil
			}
		},
	}

	cmd.Flags().StringVar(&output, "output", "table", "output format: table, json, or yaml")
	return cmd
}
