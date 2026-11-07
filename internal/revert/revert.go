// Package revert is a temporary safety net for the move to Overlord v2.
//
// Before Sync or Start first changes anything, Save copies every path they
// can change into ~/.local/state/overlord/revert. Restore puts those paths
// back and keeps whatever it replaces, so reverting never loses anything.
//
// To remove it once v2 has proven itself: delete this package,
// internal/cmd/revert.go and internal/cmd/revert_test.go, the revert.Save
// calls in internal/cmd/sync.go and internal/cmd/start.go, the two
// AddCommand lines for them in internal/cmd/root.go and the revert line in
// AGENTS.md.
package revert

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/harness"
	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/t3"
)

// target is one path Sync or Start can change.
type target struct {
	Path string
	// Entries restores each child of the directory on its own, so a child
	// created since is removed and one removed since comes back, while
	// children that belong to someone else stay.
	Entries bool
}

// Dir is where the saved copy lives.
func Dir(home string) string {
	return filepath.Join(home, ".local", "state", "overlord", "revert")
}

// Save copies every path Sync or Start can change, unless a saved copy
// already exists. It reports whether it saved one.
func Save(sys system.System) (bool, error) {
	dir := Dir(sys.Home)
	if _, err := os.Stat(dir); err == nil {
		return false, nil
	}
	cfg, err := config.Load(sys.Home)
	if err != nil {
		return false, err
	}
	targets := targetsFor(sys.Home, cfg.AgentsDir)
	// A crashed run leaves only the staging directory, so the saved copy
	// exists only once it is complete.
	staging := dir + ".saving"
	if err := os.RemoveAll(staging); err != nil {
		return false, err
	}
	for _, target := range targets {
		if err := copyTarget(target, target.Path, filepath.Join(staging, "files", target.Path)); err != nil {
			return false, err
		}
	}
	list, err := json.Marshal(targets)
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(filepath.Join(staging, "targets.json"), list, 0o644); err != nil {
		return false, err
	}
	return true, os.Rename(staging, dir)
}

// Restore puts every saved path back, moves what it replaced to the returned
// directory and removes the saved copy, so the next Sync saves a fresh one.
func Restore(sys system.System) (string, error) {
	dir := Dir(sys.Home)
	list, err := os.ReadFile(filepath.Join(dir, "targets.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("nothing to revert: no saved copy exists yet (sync or start saves one before changing anything)")
	}
	if err != nil {
		return "", err
	}
	var targets []target
	if err := json.Unmarshal(list, &targets); err != nil {
		return "", fmt.Errorf("%s: %w", dir, err)
	}
	replaced, err := os.MkdirTemp(filepath.Dir(dir), "revert-replaced-")
	if err != nil {
		return "", err
	}
	for _, target := range targets {
		paths := []string{target.Path}
		if target.Entries {
			paths, err = childPaths(target.Path, filepath.Join(dir, "files", target.Path))
			if err != nil {
				return "", err
			}
		}
		for _, path := range paths {
			if err := restorePath(path, filepath.Join(dir, "files", path), filepath.Join(replaced, path)); err != nil {
				return "", err
			}
		}
	}
	// Remove succeeds only when the directory is empty, which means nothing
	// was replaced, so the caller gets "".
	if os.Remove(replaced) == nil {
		replaced = ""
	}
	return replaced, os.RemoveAll(dir)
}

func targetsFor(home, agentsDir string) []target {
	targets := []target{
		{Path: harness.SkillsDir(home), Entries: true},
		{Path: harness.OpenCodeSkillsDir(home)},
		{Path: agentsDir},
		{Path: harness.ClaudeSettingsPath(home)},
		{Path: harness.CodexConfigPath(home)},
		{Path: harness.CodexHooksPath(home)},
		{Path: harness.OpenCodePluginPath(home)},
		{Path: t3.SettingsPath(home)},
		{Path: config.Path(home)},
	}
	for _, path := range harness.InstructionPaths(home) {
		targets = append(targets, target{Path: path})
	}
	for _, h := range harness.All {
		targets = append(targets, target{Path: harness.GlobalAgentDir(h, home)})
	}
	return targets
}

// copyTarget copies src to dst; for an Entries target it copies the children
// Overlord may change, not the whole directory.
func copyTarget(target target, src, dst string) error {
	if !target.Entries {
		return copyTree(src, dst)
	}
	children, err := childPaths(src)
	if err != nil {
		return err
	}
	for _, child := range children {
		if err := copyTree(child, filepath.Join(dst, filepath.Base(child))); err != nil {
			return err
		}
	}
	return nil
}

// childPaths lists the children of every directory in dirs (those that
// exist), by the path each has under the first directory.
func childPaths(dirs ...string) ([]string, error) {
	seen := map[string]bool{}
	var paths []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if seen[entry.Name()] || harness.ClaudeOwnedSkillEntries[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true
			paths = append(paths, filepath.Join(dirs[0], entry.Name()))
		}
	}
	return paths, nil
}

// restorePath replaces what is at path with the saved copy (or with nothing
// when there is none), first moving what is there to keep.
func restorePath(path, saved, keep string) error {
	if _, err := os.Lstat(path); err == nil {
		if err := copyTree(path, keep); err != nil {
			return err
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return copyTree(saved, path)
}

// copyTree copies the file, link or directory tree at src to dst, keeping
// links as links. A missing src copies nothing.
func copyTree(src, dst string) error {
	info, err := os.Lstat(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(target, dst)
	case info.IsDir():
		if err := os.Mkdir(dst, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	case info.Mode().IsRegular():
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, info.Mode().Perm())
	}
	return fmt.Errorf("%s: cannot copy a %s", src, info.Mode().Type())
}
