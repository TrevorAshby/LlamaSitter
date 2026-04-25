package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/trevorashby/llamasitter/internal/config"
	"github.com/trevorashby/llamasitter/internal/configedit"
	"github.com/trevorashby/llamasitter/internal/desktop"
	"gopkg.in/yaml.v3"
)

type inspectOutput string

const (
	outputTable inspectOutput = "table"
	outputJSON  inspectOutput = "json"
	outputYAML  inspectOutput = "yaml"
)

func parseInspectOutput(value string) (inspectOutput, error) {
	switch inspectOutput(strings.ToLower(strings.TrimSpace(value))) {
	case outputTable:
		return outputTable, nil
	case outputJSON:
		return outputJSON, nil
	case outputYAML:
		return outputYAML, nil
	default:
		return "", fmt.Errorf("unsupported output format %q (expected table, json, or yaml)", value)
	}
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeYAML(w io.Writer, value any) error {
	raw, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}

func writeRawYAML(w io.Writer, raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	if !bytes.HasSuffix(raw, []byte("\n")) {
		_, err := io.WriteString(w, "\n")
		return err
	}
	return nil
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != count {
			_ = cmd.Usage()
			return usageErrorf("accepts %d arg(s), received %d", count, len(args))
		}
		return nil
	}
}

func noArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 0 {
		_ = cmd.Usage()
		return usageErrorf("accepts no positional arguments")
	}
	return nil
}

func resolveConfigPath(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		path = config.DefaultPath
	}
	return filepath.Abs(path)
}

func loadConfig(_ context.Context, path string) (config.Config, string, error) {
	resolved, err := resolveConfigPath(path)
	if err != nil {
		return config.Config{}, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, resolved, err
	}
	return cfg, resolved, nil
}

func loadConfigDocument(path string) (*configedit.Document, string, error) {
	resolved, err := resolveConfigPath(path)
	if err != nil {
		return nil, "", err
	}
	doc, err := configedit.Load(path)
	if err != nil {
		return nil, resolved, err
	}
	return doc, resolved, nil
}

func restartHint(path string) string {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		if managed, err := desktop.ManagedPaths(); err == nil && sameFilePath(path, managed.Config) {
			if runtime.GOOS == "darwin" {
				return "Restart the desktop app to apply the updated config:\n  open -a LlamaSitter"
			}
			return "Restart the desktop companion to apply the updated config:\n  llamasitter-desktop --mode=dashboard"
		}
	}
	return fmt.Sprintf("Restart LlamaSitter to apply the updated config:\n  llamasitter serve --config %s", path)
}

func sameFilePath(left, right string) bool {
	l, lErr := filepath.Abs(left)
	r, rErr := filepath.Abs(right)
	if lErr != nil || rErr != nil {
		return left == right
	}
	return l == r
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
