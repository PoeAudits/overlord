// Package templates embeds the Setup manifest and the Templates Setup writes
// into Projects.
package templates

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Strategy says how Setup guarantees one manifest item.
type Strategy string

const (
	Create  Strategy = "create"
	Dir     Strategy = "dir"
	Symlink Strategy = "symlink"
)

// Item is one guaranteed path in a Project, relative to the Project root.
type Item struct {
	Path     string   `yaml:"path"`
	Strategy Strategy `yaml:"strategy"`
	// Target is the link destination for the symlink strategy.
	Target string `yaml:"target"`
}

//go:embed manifest.yaml
var manifest []byte

// Manifest returns the guaranteed items in the order Setup checks them.
func Manifest() ([]Item, error) {
	var items []Item
	if err := yaml.Unmarshal(manifest, &items); err != nil {
		return nil, fmt.Errorf("parse embedded manifest: %w", err)
	}
	return items, nil
}
