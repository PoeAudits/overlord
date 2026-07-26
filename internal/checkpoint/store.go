// Package checkpoint persists bare-terminal tmux workspace topology.
package checkpoint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"gopkg.in/yaml.v3"
)

const DefaultPath = "~/.config/overlord/workspace-checkpoints.yaml"

type Store struct {
	Version  int          `yaml:"version"`
	Current  *Checkpoint  `yaml:"current,omitempty"`
	Archives []Checkpoint `yaml:"archives,omitempty"`
}

type Checkpoint struct {
	SavedAt       time.Time `yaml:"saved_at"`
	AttachSession string    `yaml:"attach_session"`
	Sessions      []Session `yaml:"sessions"`
}

type Session struct {
	Name         string   `yaml:"name"`
	Index        int      `yaml:"index"`
	ActiveWindow int      `yaml:"active_window"`
	Windows      []Window `yaml:"windows"`
}

type Window struct {
	Name       string `yaml:"name"`
	Index      int    `yaml:"index"`
	ActivePane int    `yaml:"active_pane"`
	Layout     string `yaml:"layout"`
	Panes      []Pane `yaml:"panes"`
}

type Pane struct {
	Index int    `yaml:"index"`
	Path  string `yaml:"path"`
}

func Load(path string) (*Store, error) {
	path, err := registry.ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("expand checkpoint path: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Store{Version: 1}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read checkpoints: %w", err)
	}
	var store Store
	if err := yaml.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("parse checkpoints: %w", err)
	}
	if store.Version != 1 {
		return nil, fmt.Errorf("unsupported checkpoint version %d", store.Version)
	}
	return &store, nil
}

// Save promotes the previous current checkpoint into the two-entry archive.
func Save(path string, next *Store) error {
	if next == nil || next.Current == nil {
		return fmt.Errorf("checkpoint current cannot be nil")
	}
	path, err := registry.ExpandPath(path)
	if err != nil {
		return fmt.Errorf("expand checkpoint path: %w", err)
	}
	previous, err := Load(path)
	if err != nil {
		return err
	}
	archives := append([]Checkpoint(nil), previous.Archives...)
	if previous.Current != nil {
		archives = append([]Checkpoint{*previous.Current}, archives...)
	}
	if len(archives) > 2 {
		archives = archives[:2]
	}
	next.Version, next.Archives = 1, archives
	return write(path, next)
}

func Revert(path string, index int) error {
	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return fmt.Errorf("expand checkpoint path: %w", err)
	}
	store, err := Load(expandedPath)
	if err != nil {
		return err
	}
	if index < 0 || index >= len(store.Archives) {
		return fmt.Errorf("checkpoint archive %d does not exist", index)
	}
	selected := store.Archives[index]
	archives := append([]Checkpoint(nil), store.Archives[:index]...)
	archives = append(archives, store.Archives[index+1:]...)
	if store.Current != nil {
		archives = append([]Checkpoint{*store.Current}, archives...)
	}
	if len(archives) > 2 {
		archives = archives[:2]
	}
	store.Current, store.Archives = &selected, archives
	return write(expandedPath, store)
}

func write(path string, store *Store) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create checkpoint directory: %w", err)
	}
	data, err := yaml.Marshal(store)
	if err != nil {
		return fmt.Errorf("marshal checkpoints: %w", err)
	}
	return fileutil.AtomicWriteFile(path, data, 0o644)
}
