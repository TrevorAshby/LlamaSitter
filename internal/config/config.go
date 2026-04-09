package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultPath = "llamasitter.yaml"

type Config struct {
	Listeners []ListenerConfig `yaml:"listeners" json:"listeners"`
	Storage   StorageConfig    `yaml:"storage" json:"storage"`
	Privacy   PrivacyConfig    `yaml:"privacy" json:"privacy"`
	UI        UIConfig         `yaml:"ui" json:"ui"`
}

type ListenerConfig struct {
	Name        string            `yaml:"name" json:"name"`
	ListenAddr  string            `yaml:"listen_addr" json:"listen_addr"`
	UpstreamURL string            `yaml:"upstream_url" json:"upstream_url"`
	DefaultTags map[string]string `yaml:"default_tags" json:"default_tags"`
}

type StorageConfig struct {
	SQLitePath string `yaml:"sqlite_path" json:"sqlite_path"`
}

type PrivacyConfig struct {
	PersistBodies    bool     `yaml:"persist_bodies" json:"persist_bodies"`
	RedactHeaders    []string `yaml:"redact_headers" json:"redact_headers"`
	RedactJSONFields []string `yaml:"redact_json_fields" json:"redact_json_fields"`
}

type UIConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	ListenAddr string `yaml:"listen_addr" json:"listen_addr"`
}

func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		path = DefaultPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}

	if err := cfg.normalize(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c *Config) normalize() error {
	if c.Storage.SQLitePath == "" {
		c.Storage.SQLitePath = "~/.llamasitter/llamasitter.db"
	}

	expanded, err := expandPath(c.Storage.SQLitePath)
	if err != nil {
		return fmt.Errorf("expand storage path: %w", err)
	}
	c.Storage.SQLitePath = expanded

	if c.UI.Enabled && c.UI.ListenAddr == "" {
		c.UI.ListenAddr = "127.0.0.1:11438"
	}

	c.Privacy.RedactHeaders = normalizeList(c.Privacy.RedactHeaders)
	c.Privacy.RedactJSONFields = normalizeList(c.Privacy.RedactJSONFields)

	return c.Validate()
}

func (c Config) Validate() error {
	if len(c.Listeners) == 0 {
		return errors.New("config must define at least one listener")
	}

	names := map[string]struct{}{}
	addrs := map[string]struct{}{}

	for i, listener := range c.Listeners {
		if listener.Name == "" {
			return fmt.Errorf("listener %d: name is required", i)
		}
		if _, exists := names[listener.Name]; exists {
			return fmt.Errorf("listener %q: duplicate name", listener.Name)
		}
		names[listener.Name] = struct{}{}

		if err := validateAddr(listener.ListenAddr); err != nil {
			return fmt.Errorf("listener %q: invalid listen_addr: %w", listener.Name, err)
		}
		if _, exists := addrs[listener.ListenAddr]; exists {
			return fmt.Errorf("listener %q: duplicate listen_addr %q", listener.Name, listener.ListenAddr)
		}
		addrs[listener.ListenAddr] = struct{}{}

		parsed, err := url.Parse(listener.UpstreamURL)
		if err != nil {
			return fmt.Errorf("listener %q: invalid upstream_url: %w", listener.Name, err)
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("listener %q: upstream_url must use http or https", listener.Name)
		}
		if parsed.Host == "" {
			return fmt.Errorf("listener %q: upstream_url host is required", listener.Name)
		}
	}

	if c.Storage.SQLitePath == "" {
		return errors.New("storage.sqlite_path is required")
	}

	if c.UI.Enabled {
		if err := validateAddr(c.UI.ListenAddr); err != nil {
			return fmt.Errorf("ui.listen_addr: %w", err)
		}
	}

	return nil
}

func defaultConfig() Config {
	return Config{
		Storage: StorageConfig{
			SQLitePath: "~/.llamasitter/llamasitter.db",
		},
		Privacy: PrivacyConfig{
			RedactHeaders: []string{
				"authorization",
				"proxy-authorization",
			},
			RedactJSONFields: []string{
				"prompt",
				"messages",
			},
		},
		UI: UIConfig{
			Enabled:    true,
			ListenAddr: "127.0.0.1:11438",
		},
	}
}

func expandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	return filepath.Clean(path), nil
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func validateAddr(addr string) error {
	if strings.TrimSpace(addr) == "" {
		return errors.New("must not be empty")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return err
	}
	return nil
}
