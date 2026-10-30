// Package project implements Setup for one Project: checking it against the
// embedded manifest and repairing what Overlord owns.
package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
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
	Path string `json:"path"`
	// Block names the Managed Block for the block strategy.
	Block    string             `json:"block,omitempty"`
	Strategy templates.Strategy `json:"strategy"`
	Status   Status             `json:"status"`
	// Detail says why an item is in conflict.
	Detail string `json:"detail,omitempty"`
}

// Check reports the status of every manifest item in the Project checkout at
// root without changing anything. Languages are detected in projectRoot, the
// Project's main checkout, which may differ from root when root is a worktree.
func Check(ctx context.Context, runner system.Runner, root, projectRoot string) (Plan, error) {
	_, _, plan, err := check(ctx, runner, root, projectRoot)
	return plan, err
}

// check returns the manifest items that apply to this Project, the Template
// data and the plan, with the items and plan rows in the same order.
func check(ctx context.Context, runner system.Runner, root, projectRoot string) ([]templates.Item, templates.Data, Plan, error) {
	manifest, err := templates.Manifest()
	if err != nil {
		return nil, templates.Data{}, Plan{}, err
	}
	data, err := templateData(ctx, runner, root, projectRoot)
	if err != nil {
		return nil, templates.Data{}, Plan{}, err
	}
	var items []templates.Item
	plan := Plan{Root: root}
	for _, item := range manifest {
		if item.Language != "" && !slices.Contains(data.Languages, item.Language) {
			continue
		}
		status, detail, err := itemStatus(root, item, data)
		if err != nil {
			return nil, templates.Data{}, Plan{}, fmt.Errorf("check %s: %w", item.Path, err)
		}
		items = append(items, item)
		plan.Items = append(plan.Items, Item{Path: item.Path, Block: item.Block, Strategy: item.Strategy, Status: status, Detail: detail})
	}
	return items, data, plan, nil
}

func itemStatus(root string, item templates.Item, data templates.Data) (Status, string, error) {
	path := filepath.Join(root, item.Path)
	if item.Strategy == templates.Block {
		return blockStatus(path, item, data)
	}
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

// templateData names the Project after its GitHub origin repo, falling back
// to the directory name when there is no GitHub origin.
func templateData(ctx context.Context, runner system.Runner, root, projectRoot string) (templates.Data, error) {
	languages, err := detectLanguages(projectRoot)
	if err != nil {
		return templates.Data{}, err
	}
	data := templates.Data{ProjectName: filepath.Base(root), Version: templates.Version, Languages: languages}
	output, err := runner.Run(ctx, root, "git", "remote", "get-url", "origin")
	if err != nil {
		return data, nil
	}
	owner, repo, ok := parseGitHubRemote(strings.TrimSpace(string(output)))
	if !ok {
		return data, nil
	}
	data.ProjectName, data.Owner, data.Repo = repo, owner, repo
	return data, nil
}

// detectLanguages lists each language in templates.Languages whose marker
// file exists in projectRoot.
func detectLanguages(projectRoot string) ([]string, error) {
	var found []string
	for _, language := range templates.Languages {
		for _, marker := range language.Markers {
			_, err := os.Stat(filepath.Join(projectRoot, marker))
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			found = append(found, language.Name)
			break
		}
	}
	return found, nil
}

func parseGitHubRemote(url string) (owner, repo string, ok bool) {
	for _, prefix := range []string{"git@github.com:", "ssh://git@github.com/", "https://github.com/", "http://github.com/"} {
		rest, found := strings.CutPrefix(url, prefix)
		if !found {
			continue
		}
		owner, repo, found = strings.Cut(strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git"), "/")
		if !found || owner == "" || repo == "" || strings.Contains(repo, "/") {
			return "", "", false
		}
		return owner, repo, true
	}
	return "", "", false
}
