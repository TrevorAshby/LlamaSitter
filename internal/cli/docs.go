package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func GenerateReferenceDocs(dir string) error {
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	root := newRootCommand(context.Background(), logger, io.Discard, io.Discard)
	return writeCommandDocs(root, filepath.Clean(dir))
}

func writeCommandDocs(root *cobra.Command, dir string) error {
	var visit func(*cobra.Command) error
	visit = func(cmd *cobra.Command) error {
		if cmd.Name() != "help" && !cmd.Hidden {
			filename := strings.ReplaceAll(cmd.CommandPath(), " ", "_") + ".md"
			if err := os.WriteFile(filepath.Join(dir, filename), []byte(commandDoc(cmd)), 0o644); err != nil {
				return err
			}
		}
		for _, child := range cmd.Commands() {
			if child == nil || child.Hidden || child.Name() == "help" || !child.IsAvailableCommand() {
				continue
			}
			if err := visit(child); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(root)
}

func commandDoc(cmd *cobra.Command) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# %s\n\n", cmd.CommandPath())
	if cmd.Short != "" {
		fmt.Fprintf(&buf, "%s\n\n", cmd.Short)
	}
	if long := strings.TrimSpace(cmd.Long); long != "" && long != cmd.Short {
		fmt.Fprintf(&buf, "%s\n\n", long)
	}
	if example := strings.TrimSpace(cmd.Example); example != "" {
		fmt.Fprintf(&buf, "## Examples\n\n```text\n%s\n```\n\n", example)
	}
	fmt.Fprintf(&buf, "## Usage\n\n```text\n%s```\n", cmd.UsageString())
	return buf.String()
}
