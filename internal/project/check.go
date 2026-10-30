// Package project implements Setup for one Project: checking it against the
// embedded manifest and, later, repairing what Overlord owns.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/templates"
)

// Status is the state of one guaranteed item.
type Status string

const (
	Missing Status = "missing"
	Stale   Status = "stale"
	OK      Status = "ok"
)

// Plan is the result of checking a Project.
type Plan struct {
	Root  string `json:"root"`
	Items []Item `json:"items"`
}

// Item is one guaranteed path and its current status.
type Item struct {
	Path     string             `json:"path"`
	Strategy templates.Strategy `json:"strategy"`
	Status   Status             `json:"status"`
}

// Check reports the status of every manifest item in the Project at root
// without changing anything.
func Check(root string) (Plan, error) {
	manifest, err := templates.Manifest()
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{Root: root}
	for _, item := range manifest {
		status, err := itemStatus(filepath.Join(root, item.Path), item)
		if err != nil {
			return Plan{}, fmt.Errorf("check %s: %w", item.Path, err)
		}
		plan.Items = append(plan.Items, Item{Path: item.Path, Strategy: item.Strategy, Status: status})
	}
	return plan, nil
}

func itemStatus(path string, item templates.Item) (Status, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Missing, nil
	}
	if err != nil {
		return "", err
	}
	switch item.Strategy {
	case templates.Create:
		return OK, nil
	case templates.Dir:
		if info.IsDir() {
			return OK, nil
		}
		return Stale, nil
	case templates.Symlink:
		target, err := os.Readlink(path)
		if err != nil || target != item.Target {
			return Stale, nil
		}
		return OK, nil
	}
	return "", fmt.Errorf("unknown strategy %q", item.Strategy)
}
