// Package claude registers VMs as SSH connections in Claude Desktop, through
// the "sshConfigs" list in ~/.claude/settings.json. It only touches entries
// whose id starts with "limabean-", keeps every other setting and the order
// of top-level keys, and leaves a file that isn't valid JSON unchanged.
package claude

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IDPrefix starts the id of every entry limabean owns.
const IDPrefix = "limabean-"

// Entry is one SSH connection.
type Entry struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	SSHHost string `json:"sshHost"`
}

// EntryFor returns the connection for a VM: "dev (limabean)" on host lima-dev.
func EntryFor(vm string) Entry {
	return Entry{ID: IDPrefix + vm, Name: vm + " (limabean)", SSHHost: "lima-" + vm}
}

// Settings is a settings.json file.
type Settings struct{ Path string }

// DefaultPath is ~/.claude/settings.json.
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

// ErrInvalid means the file isn't a JSON object; it's left as it is.
var ErrInvalid = errors.New("settings file isn't valid JSON")

// VMs returns the names of the VMs with a limabean entry.
func (s Settings) VMs() ([]string, error) {
	doc, err := s.load()
	if err != nil {
		return nil, err
	}
	entries, err := doc.entries()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, raw := range entries {
		if id := entryID(raw); strings.HasPrefix(id, IDPrefix) {
			names = append(names, strings.TrimPrefix(id, IDPrefix))
		}
	}
	return names, nil
}

// Add adds or replaces the VM's entry.
func (s Settings) Add(vm string) error {
	return s.edit(func(entries []json.RawMessage) ([]json.RawMessage, error) {
		e := EntryFor(vm)
		raw, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		return append(without(entries, e.ID), raw), nil
	})
}

// Remove deletes the VM's entry, if there is one.
func (s Settings) Remove(vm string) error {
	return s.edit(func(entries []json.RawMessage) ([]json.RawMessage, error) {
		return without(entries, IDPrefix+vm), nil
	})
}

func without(entries []json.RawMessage, id string) []json.RawMessage {
	out := entries[:0:0]
	for _, raw := range entries {
		if entryID(raw) != id {
			out = append(out, raw)
		}
	}
	return out
}

func entryID(raw json.RawMessage) string {
	var e struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(raw, &e)
	return e.ID
}

func (s Settings) edit(fn func([]json.RawMessage) ([]json.RawMessage, error)) error {
	doc, err := s.load()
	if err != nil {
		return err
	}
	entries, err := doc.entries()
	if err != nil {
		return err
	}
	entries, err = fn(entries)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(entries)
	if err != nil {
		return err
	}
	doc.set("sshConfigs", raw)
	out, err := doc.marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return err
	}
	// Write in place rather than renaming, so the file's permissions and any
	// symlink (e.g. into a dotfiles repo) are kept.
	f, err := os.OpenFile(s.Path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(out); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// document is a JSON object whose top-level key order is preserved.
type document struct {
	keys   []string
	values map[string]json.RawMessage
}

func (s Settings) load() (*document, error) {
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) || (err == nil && len(bytes.TrimSpace(data)) == 0) {
		return &document{values: map[string]json.RawMessage{}}, nil
	}
	if err != nil {
		return nil, err
	}
	doc, err := parseObject(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", s.Path, ErrInvalid)
	}
	return doc, nil
}

func parseObject(data []byte) (*document, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		return nil, ErrInvalid
	}
	doc := &document{values: map[string]json.RawMessage{}}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, ErrInvalid
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		if _, dup := doc.values[key]; !dup {
			doc.keys = append(doc.keys, key)
		}
		doc.values[key] = raw
	}
	if _, err := dec.Token(); err != nil { // closing brace
		return nil, err
	}
	if dec.More() {
		return nil, ErrInvalid
	}
	return doc, nil
}

func (d *document) entries() ([]json.RawMessage, error) {
	raw, ok := d.values["sshConfigs"]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, errors.New(`"sshConfigs" in the settings file isn't a list; left it unchanged`)
	}
	return entries, nil
}

func (d *document) set(key string, raw json.RawMessage) {
	if _, ok := d.values[key]; !ok {
		d.keys = append(d.keys, key)
	}
	d.values[key] = raw
}

func (d *document) marshal() ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("{")
	for i, k := range d.keys {
		if i > 0 {
			b.WriteString(",")
		}
		key, _ := json.Marshal(k)
		b.WriteString("\n  ")
		b.Write(key)
		b.WriteString(": ")
		var v bytes.Buffer
		if err := json.Indent(&v, d.values[k], "  ", "  "); err != nil {
			return nil, err
		}
		b.Write(v.Bytes())
	}
	if len(d.keys) > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes(), nil
}
