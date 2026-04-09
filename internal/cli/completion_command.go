package cli

import "github.com/spf13/cobra"

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Args:  exactArgs(1),
		Example: "  llamasitter completion zsh > ~/.zsh/completions/_llamasitter\n" +
			"  llamasitter completion bash > /usr/local/etc/bash_completion.d/llamasitter",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				_ = cmd.Usage()
				return usageErrorf("unsupported shell %q (expected bash, zsh, fish, or powershell)", args[0])
			}
		},
	}

	return cmd
}
