// Package machine applies the agents directory to every Harness on this
// machine (Sync) and brings up the services T3 Code needs (Start).
package machine

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/catalog"
	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/t3"
)

// Action is what Sync did, or in a check would do, to one path.
type Action string

const (
	OK      Action = "ok"
	Create  Action = "create"
	Remove  Action = "remove"
	Replace Action = "replace"
	// Conflict marks a path Sync must own but will not change because that
	// would lose content. It makes a check fail.
	Conflict Action = "conflict"
	// Shadowed marks a path Sync leaves alone that hides a catalog skill.
	Shadowed Action = "shadowed"
)

// Result is the outcome of one Sync step for one path.
type Result struct {
	Step   string
	Path   string
	Action Action
	Detail string
}

// Report lists the results of every step in order.
type Report struct {
	Results []Result
}

// Conflicted reports whether any path is in conflict.
func (r Report) Conflicted() bool {
	for _, result := range r.Results {
		if result.Action == Conflict {
			return true
		}
	}
	return false
}

// syncRun is the state every step receives.
type syncRun struct {
	sys       system.System
	agentsDir string
	// check means report only: a step must not change anything.
	check bool
}

// step is one ordered part of Sync. Later steps (Roles, Models, hooks, T3)
// are added by appending to steps.
type step struct {
	name string
	run  func(syncRun) ([]Result, error)
}

var steps = []step{
	{name: "skills", run: syncSkills},
	{name: "instructions", run: syncInstructions},
	{name: "t3", run: syncT3},
}

// Sync applies agentsDir to every Harness under sys.Home. With check it only
// reports what it would do.
func Sync(sys system.System, agentsDir string, check bool) (Report, error) {
	run := syncRun{sys: sys, agentsDir: agentsDir, check: check}
	var report Report
	for _, step := range steps {
		results, err := step.run(run)
		if err != nil {
			return report, fmt.Errorf("sync %s: %w", step.name, err)
		}
		for _, result := range results {
			result.Step = step.name
			report.Results = append(report.Results, result)
		}
	}
	return report, nil
}

func syncSkills(run syncRun) ([]Result, error) {
	skills, err := catalog.LoadSkills(run.agentsDir)
	if err != nil {
		return nil, err
	}
	linksDir := filepath.Join(run.sys.Home, ".claude", "skills")
	var results []Result
	for _, skill := range skills {
		if protectedSkillEntries[skill.Name] {
			continue
		}
		path := filepath.Join(linksDir, skill.Name)
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			results = append(results, Result{Path: path, Action: Create})
			if err := run.createLink(path, skill.Dir); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		case isLinkTo(path, skill.Dir):
			results = append(results, Result{Path: path, Action: OK})
		case info.IsDir():
			results = append(results, Result{Path: path, Action: Shadowed, Detail: "a real directory hides the catalog skill"})
		default:
			results = append(results, Result{Path: path, Action: Shadowed, Detail: "something other than a link to the catalog skill is here"})
		}
	}
	removed, err := run.removeDanglingSkillLinks(linksDir)
	if err != nil {
		return nil, err
	}
	return append(results, removed...), nil
}

// removeDanglingSkillLinks removes links in linksDir that point into the
// catalog's skills directory at a skill that no longer exists. Links pointing
// anywhere else are not Overlord's and stay.
func (run syncRun) removeDanglingSkillLinks(linksDir string) ([]Result, error) {
	entries, err := os.ReadDir(linksDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	skillsDir := catalog.SkillsDir(run.agentsDir)
	var results []Result
	for _, entry := range entries {
		path := filepath.Join(linksDir, entry.Name())
		if protectedSkillEntries[entry.Name()] || entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		target, err := os.Readlink(path)
		if err != nil {
			return nil, err
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(linksDir, target)
		}
		if filepath.Dir(target) != skillsDir {
			continue
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		results = append(results, Result{Path: path, Action: Remove})
		if run.check {
			continue
		}
		if err := os.Remove(path); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// protectedSkillEntries are entries of ~/.claude/skills that belong to Claude
// Code itself (claude.ai skill sync and its trash). Sync never touches them.
var protectedSkillEntries = map[string]bool{"synced": true, ".trash": true}

func syncInstructions(run syncRun) ([]Result, error) {
	source := catalog.InstructionsPath(run.agentsDir)
	var results []Result
	for _, path := range []string{
		filepath.Join(run.sys.Home, ".claude", "CLAUDE.md"),
		filepath.Join(run.sys.Home, ".codex", "AGENTS.md"),
		filepath.Join(run.sys.Home, ".config", "opencode", "AGENTS.md"),
	} {
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, os.ErrNotExist):
			results = append(results, Result{Path: path, Action: Create})
			if err := run.createLink(path, source); err != nil {
				return nil, err
			}
		case err != nil:
			return nil, err
		case isLinkTo(path, source):
			results = append(results, Result{Path: path, Action: OK})
		case !info.Mode().IsRegular():
			results = append(results, Result{Path: path, Action: Conflict, Detail: "is not a link to the catalog's AGENTS.md; left in place"})
		default:
			result, err := run.replaceIdenticalFile(path, source)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
		}
	}
	return results, nil
}

// replaceIdenticalFile replaces the regular file at path with a link to
// source when both have the same content. A differing file holds rules the
// user has not merged into the catalog yet, so it is left for them.
func (run syncRun) replaceIdenticalFile(path, source string) (Result, error) {
	current, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	want, err := os.ReadFile(source)
	if err != nil {
		return Result{}, err
	}
	if !bytes.Equal(current, want) {
		return Result{Path: path, Action: Conflict, Detail: "differs from the catalog's AGENTS.md; left in place, merge it by hand"}, nil
	}
	if run.check {
		return Result{Path: path, Action: Replace}, nil
	}
	// Link beside the file, then rename over it, so path is never missing.
	staged := path + ".overlord-sync"
	if err := os.Symlink(source, staged); err != nil {
		return Result{}, err
	}
	return Result{Path: path, Action: Replace}, os.Rename(staged, path)
}

func isLinkTo(path, target string) bool {
	current, err := os.Readlink(path)
	return err == nil && current == target
}

// createLink creates path as a symlink to target, unless this is a check.
func (run syncRun) createLink(path, target string) error {
	if run.check {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.Symlink(target, path)
}

// syncT3 ensures T3 Code runs `overlord setup` on every new worktree.
func syncT3(run syncRun) ([]Result, error) {
	status, err := t3.EnsureSetupScript(run.sys.Home, run.check)
	if err != nil {
		return nil, err
	}
	return []Result{{Path: t3.SettingsPath(run.sys.Home), Action: t3Actions[status], Detail: "overlord-setup script"}}, nil
}

var t3Actions = map[t3.Status]Action{t3.Missing: Create, t3.Stale: Replace, t3.OK: OK}
