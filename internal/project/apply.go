package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// Apply checks the Project at root, then creates or refreshes every missing
// or stale item. It returns the plan as it was before the repairs; conflict
// items are left untouched. An error means a Template failed to render or a
// path could not be written.
func Apply(ctx context.Context, runner system.Runner, root string) (Plan, error) {
	manifest, plan, err := check(root)
	if err != nil {
		return Plan{}, err
	}
	data := templateData(ctx, runner, root)
	for i, item := range manifest {
		if status := plan.Items[i].Status; status != Missing && status != Stale {
			continue
		}
		if err := repair(filepath.Join(root, item.Path), item, data); err != nil {
			return plan, fmt.Errorf("set up %s: %w", item.Path, err)
		}
	}
	return plan, nil
}

func repair(path string, item templates.Item, data templates.Data) error {
	switch item.Strategy {
	case templates.Create:
		content, err := templates.Render(item.Template, data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, content, 0o644)
	case templates.Dir:
		return repairDir(path)
	case templates.Symlink:
		// Check only reports stale for a wrong link or an empty directory,
		// both of which Remove can clear.
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.Symlink(item.Target, path)
	}
	return fmt.Errorf("unknown strategy %q", item.Strategy)
}

// repairDir ensures the directory exists with a .gitkeep only while empty.
func repairDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	empty, hasKeep, err := dirContents(path)
	if err != nil {
		return err
	}
	keep := filepath.Join(path, ".gitkeep")
	if empty && !hasKeep {
		return os.WriteFile(keep, nil, 0o644)
	}
	if !empty && hasKeep {
		return os.Remove(keep)
	}
	return nil
}

// templateData names the Project after its GitHub origin repo, falling back
// to the directory name when there is no GitHub origin.
func templateData(ctx context.Context, runner system.Runner, root string) templates.Data {
	data := templates.Data{ProjectName: filepath.Base(root), Version: templates.Version}
	output, err := runner.Run(ctx, root, "git", "remote", "get-url", "origin")
	if err != nil {
		return data
	}
	owner, repo, ok := parseGitHubRemote(strings.TrimSpace(string(output)))
	if !ok {
		return data
	}
	data.ProjectName, data.Owner, data.Repo = repo, owner, repo
	return data
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
