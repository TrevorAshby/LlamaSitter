package configedit

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/trevorashby/llamasitter/internal/config"
	"gopkg.in/yaml.v3"
)

type Document struct {
	node yaml.Node
}

type ListenerUpdate struct {
	Rename      *string
	ListenAddr  *string
	UpstreamURL *string
}

func DefaultYAML() string {
	return strings.TrimSpace(`
listeners:
  - name: default
    listen_addr: "127.0.0.1:11435"
    upstream_url: "http://127.0.0.1:11434"
    default_tags:
      client_type: "unknown"
      client_instance: "default"

storage:
  sqlite_path: "~/.llamasitter/llamasitter.db"

privacy:
  persist_bodies: false
  redact_headers:
    - authorization
    - proxy-authorization
  redact_json_fields:
    - prompt
    - messages

ui:
  enabled: true
  listen_addr: "127.0.0.1:11438"
`) + "\n"
}

func NewDefault() *Document {
	doc, err := Parse([]byte(DefaultYAML()))
	if err != nil {
		panic(err)
	}
	return doc
}

func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	return Parse(raw)
}

func Parse(raw []byte) (*Document, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, fmt.Errorf("decode config: empty document")
	}

	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	doc := &Document{node: node}
	if _, err := doc.root(); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *Document) Config() (config.Config, error) {
	raw, err := d.Bytes()
	if err != nil {
		return config.Config{}, err
	}
	return config.Parse(raw)
}

func (d *Document) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&d.node); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (d *Document) WriteAtomic(path string) error {
	raw, err := d.Bytes()
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if stat, err := os.Stat(path); err == nil {
		mode = stat.Mode().Perm()
	}

	file, err := os.CreateTemp(dir, ".llamasitter-*.yaml")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (d *Document) AddListener(listener config.ListenerConfig) error {
	listeners, err := d.ensureSectionSequence("listeners")
	if err != nil {
		return err
	}
	if _, existing := findListener(listeners, listener.Name); existing != nil {
		return fmt.Errorf("listener %q already exists", listener.Name)
	}
	listeners.Content = append(listeners.Content, listenerNode(listener))
	return nil
}

func (d *Document) UpdateListener(name string, update ListenerUpdate) error {
	listeners, err := d.ensureSectionSequence("listeners")
	if err != nil {
		return err
	}
	index, listener := findListener(listeners, name)
	if listener == nil {
		return fmt.Errorf("listener %q not found", name)
	}

	if update.Rename != nil && *update.Rename != name {
		if existingIndex, _ := findListener(listeners, *update.Rename); existingIndex >= 0 {
			return fmt.Errorf("listener %q already exists", *update.Rename)
		}
		setMappingValue(listener, "name", newStringNode(*update.Rename))
	}
	if update.ListenAddr != nil {
		setMappingValue(listener, "listen_addr", newStringNode(*update.ListenAddr))
	}
	if update.UpstreamURL != nil {
		setMappingValue(listener, "upstream_url", newStringNode(*update.UpstreamURL))
	}

	listeners.Content[index] = listener
	return nil
}

func (d *Document) RemoveListener(name string) error {
	listeners, err := d.ensureSectionSequence("listeners")
	if err != nil {
		return err
	}
	index, _ := findListener(listeners, name)
	if index < 0 {
		return fmt.Errorf("listener %q not found", name)
	}
	if len(listeners.Content) <= 1 {
		return fmt.Errorf("cannot remove listener %q: config must keep at least one listener", name)
	}
	listeners.Content = append(listeners.Content[:index], listeners.Content[index+1:]...)
	return nil
}

func (d *Document) SetListenerTag(name, key, value string) error {
	listeners, err := d.ensureSectionSequence("listeners")
	if err != nil {
		return err
	}
	_, listener := findListener(listeners, name)
	if listener == nil {
		return fmt.Errorf("listener %q not found", name)
	}

	tags, err := ensureMappingValue(listener, "default_tags")
	if err != nil {
		return err
	}
	setMappingValue(tags, key, newStringNode(value))
	return nil
}

func (d *Document) UnsetListenerTag(name, key string) error {
	listeners, err := d.ensureSectionSequence("listeners")
	if err != nil {
		return err
	}
	_, listener := findListener(listeners, name)
	if listener == nil {
		return fmt.Errorf("listener %q not found", name)
	}

	tags := mappingValue(listener, "default_tags")
	if tags == nil || tags.Kind != yaml.MappingNode {
		return nil
	}
	deleteMappingValue(tags, key)
	if len(tags.Content) == 0 {
		deleteMappingValue(listener, "default_tags")
	}
	return nil
}

func (d *Document) SetUIEnabled(enabled bool) error {
	ui, err := d.ensureSectionMapping("ui")
	if err != nil {
		return err
	}
	setMappingValue(ui, "enabled", newBoolNode(enabled))
	return nil
}

func (d *Document) SetUIListenAddr(addr string) error {
	ui, err := d.ensureSectionMapping("ui")
	if err != nil {
		return err
	}
	setMappingValue(ui, "listen_addr", newStringNode(addr))
	return nil
}

func (d *Document) SetStorageSQLitePath(path string) error {
	storage, err := d.ensureSectionMapping("storage")
	if err != nil {
		return err
	}
	setMappingValue(storage, "sqlite_path", newStringNode(path))
	return nil
}

func (d *Document) SetPersistBodies(enabled bool) error {
	privacy, err := d.ensureSectionMapping("privacy")
	if err != nil {
		return err
	}
	setMappingValue(privacy, "persist_bodies", newBoolNode(enabled))
	return nil
}

func (d *Document) AddRedactHeader(name string) error {
	return d.addSequenceValue("privacy", "redact_headers", normalizeListValue(name))
}

func (d *Document) RemoveRedactHeader(name string) error {
	return d.removeSequenceValue("privacy", "redact_headers", normalizeListValue(name))
}

func (d *Document) AddRedactJSONField(name string) error {
	return d.addSequenceValue("privacy", "redact_json_fields", normalizeListValue(name))
}

func (d *Document) RemoveRedactJSONField(name string) error {
	return d.removeSequenceValue("privacy", "redact_json_fields", normalizeListValue(name))
}

func (d *Document) addSequenceValue(sectionKey, listKey, value string) error {
	if value == "" {
		return fmt.Errorf("%s must not be empty", listKey)
	}

	section, err := d.ensureSectionMapping(sectionKey)
	if err != nil {
		return err
	}
	list, err := ensureSequenceValue(section, listKey)
	if err != nil {
		return err
	}
	for _, item := range list.Content {
		if item.Value == value {
			return nil
		}
	}
	list.Content = append(list.Content, newStringNode(value))
	return nil
}

func (d *Document) removeSequenceValue(sectionKey, listKey, value string) error {
	section, err := d.ensureSectionMapping(sectionKey)
	if err != nil {
		return err
	}
	list := mappingValue(section, listKey)
	if list == nil || list.Kind != yaml.SequenceNode {
		return nil
	}

	filtered := list.Content[:0]
	for _, item := range list.Content {
		if item.Value == value {
			continue
		}
		filtered = append(filtered, item)
	}
	list.Content = filtered
	if len(list.Content) == 0 {
		deleteMappingValue(section, listKey)
	}
	return nil
}

func (d *Document) ensureSectionMapping(key string) (*yaml.Node, error) {
	root, err := d.root()
	if err != nil {
		return nil, err
	}
	return ensureMappingValue(root, key)
}

func (d *Document) ensureSectionSequence(key string) (*yaml.Node, error) {
	root, err := d.root()
	if err != nil {
		return nil, err
	}
	return ensureSequenceValue(root, key)
}

func (d *Document) root() (*yaml.Node, error) {
	if d.node.Kind == 0 {
		d.node = yaml.Node{Kind: yaml.DocumentNode}
	}
	if d.node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("config root must be a YAML document")
	}
	if len(d.node.Content) == 0 {
		d.node.Content = []*yaml.Node{newMappingNode()}
	}
	root := d.node.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("config root must be a mapping")
	}
	return root, nil
}

func ensureMappingValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	value := mappingValue(mapping, key)
	if value == nil {
		value = newMappingNode()
		setMappingValue(mapping, key, value)
		return value, nil
	}
	if value.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must be a mapping", key)
	}
	return value, nil
}

func ensureSequenceValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	value := mappingValue(mapping, key)
	if value == nil {
		value = newSequenceNode()
		setMappingValue(mapping, key, value)
		return value, nil
	}
	if value.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("%s must be a sequence", key)
	}
	return value, nil
}

func findListener(listeners *yaml.Node, name string) (int, *yaml.Node) {
	if listeners == nil || listeners.Kind != yaml.SequenceNode {
		return -1, nil
	}
	for index, item := range listeners.Content {
		if item.Kind != yaml.MappingNode {
			continue
		}
		if listenerName := mappingValue(item, "name"); listenerName != nil && listenerName.Value == name {
			return index, item
		}
	}
	return -1, nil
}

func listenerNode(listener config.ListenerConfig) *yaml.Node {
	node := newMappingNode()
	setMappingValue(node, "name", newStringNode(listener.Name))
	setMappingValue(node, "listen_addr", newStringNode(listener.ListenAddr))
	setMappingValue(node, "upstream_url", newStringNode(listener.UpstreamURL))
	if len(listener.DefaultTags) > 0 {
		tags := newMappingNode()
		keys := make([]string, 0, len(listener.DefaultTags))
		for key := range listener.DefaultTags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			setMappingValue(tags, key, newStringNode(listener.DefaultTags[key]))
		}
		setMappingValue(node, "default_tags", tags)
	}
	return node
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func setMappingValue(mapping *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content[i+1] = value
			return
		}
	}
	mapping.Content = append(mapping.Content, newStringNode(key), value)
}

func deleteMappingValue(mapping *yaml.Node, key string) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func newMappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func newSequenceNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
}

func newStringNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func newBoolNode(value bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(value)}
}

func normalizeListValue(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
