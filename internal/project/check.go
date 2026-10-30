// Package project implements Setup for one Project: checking it against the
// embedded manifest and repairing what Overlord owns.
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
	// Conflict means something Overlord does not own occupies the path, so
	// Setup reports it and leaves it untouched.
	Conflict Status = "conflict"
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
	// Detail says why an item is in conflict.
	Detail string `json:"detail,omitempty"`
}

// Check reports the status of every manifest item in the Project at root
// without changing anything.
func Check(root string) (Plan, error) {
	_, plan, err := check(root)
	return plan, err
}

func check(root string) ([]templates.Item, Plan, error) {
	manifest, err := templates.Manifest()
	if err != nil {
		return nil, Plan{}, err
	}
	plan := Plan{Root: root}
	for _, item := range manifest {
		status, detail, err := itemStatus(root, item)
		if err != nil {
			return nil, Plan{}, fmt.Errorf("check %s: %w", item.Path, err)
		}
		plan.Items = append(plan.Items, Item{Path: item.Path, Strategy: item.Strategy, Status: status, Detail: detail})
	}
	return manifest, plan, nil
}

func itemStatus(root string, item templates.Item) (Status, string, error) {
	path := filepath.Join(root, item.Path)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return absentStatus(root, item)
	}
	if err != nil {
		return "", "", err
	}
	switch item.Strategy {
	case templates.Create:
		return OK, "", nil
	case templates.Dir:
		if !info.IsDir() {
			return Conflict, "not a directory; left untouched", nil
		}
		empty, hasKeep, err := dirContents(path)
		if err != nil {
			return "", "", err
		}
		if empty != hasKeep {
			return Stale, "", nil
		}
		return OK, "", nil
	case templates.Symlink:
		return symlinkStatus(path, info, item.Target)
	}
	return "", "", fmt.Errorf("unknown strategy %q", item.Strategy)
}

// absentStatus is Missing, except for a create item whose skipIfAny glob
// already matches something (the starter ADR when any ADR exists).
func absentStatus(root string, item templates.Item) (Status, string, error) {
	if item.Strategy != templates.Create || item.SkipIfAny == "" {
		return Missing, "", nil
	}
	matches, err := filepath.Glob(filepath.Join(root, item.SkipIfAny))
	if err != nil {
		return "", "", err
	}
	if len(matches) > 0 {
		return OK, "", nil
	}
	return Missing, "", nil
}

func symlinkStatus(path string, info fs.FileInfo, target string) (Status, string, error) {
	if info.Mode()&fs.ModeSymlink != 0 {
		current, err := os.Readlink(path)
		if err != nil {
			return "", "", err
		}
		if current != target {
			return Stale, "", nil
		}
		return OK, "", nil
	}
	if !info.IsDir() {
		return Conflict, "not a symlink; left untouched", nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", "", err
	}
	if len(entries) > 0 {
		return Conflict, "real directory with content; left untouched", nil
	}
	return Stale, "", nil
}

// dirContents reports whether the directory holds nothing but an optional
// .gitkeep, and whether that .gitkeep is present.
func dirContents(path string) (empty, hasKeep bool, err error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, false, err
	}
	others := 0
	for _, entry := range entries {
		if entry.Name() == ".gitkeep" {
			hasKeep = true
			continue
		}
		others++
	}
	return others == 0, hasKeep, nil
}
