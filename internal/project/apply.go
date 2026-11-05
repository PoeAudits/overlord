package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// Apply checks the Project at root, then tries to create or refresh every
// missing or stale item, carrying on past any that fail. It returns the plan
// as it was before the repairs, except that an item that could not be written
// becomes failed with its error as the detail, a labels item whose creation
// failed becomes a warning, and a repo item in conflict becomes failed, since
// the Project is left without an origin. Other conflict items are left
// untouched. Once the repo item has created the origin, the labels are checked
// again against it. An error means the Project could not be checked at all. A
// non-nil sleep makes a failing label lookup retry, for a repository GitHub
// has only just created. owner is as for Check.
func Apply(ctx context.Context, runner system.Runner, sleep func(time.Duration), root, projectRoot, owner string) (Plan, error) {
	items, data, plan, err := check(ctx, runner, sleep, root, projectRoot, owner)
	if err != nil {
		return Plan{}, err
	}
	created := false
	for i, item := range items {
		if item.Strategy == templates.Labels && created {
			plan.Items[i].Status, plan.Items[i].Detail, plan.Items[i].MissingLabels = labelsStatus(ctx, runner, sleep, root, item, data)
		}
		status := plan.Items[i].Status
		if item.Strategy == templates.Repo && status == Conflict {
			plan.Items[i].Status = Failed
		}
		if status != Missing && status != Stale {
			continue
		}
		switch item.Strategy {
		case templates.Labels:
			plan.Items[i] = createLabels(ctx, runner, root, item, data, plan.Items[i])
			continue
		case templates.Repo:
			plan.Items[i] = createRepo(ctx, runner, root, plan.Items[i])
			if created = plan.Items[i].Status != Failed; created {
				data.Owner, data.Repo, _ = strings.Cut(plan.Items[i].repo, "/")
			}
			continue
		}
		if err := repair(filepath.Join(root, item.Path), item, data, earlierBlocks(items[:i], item.Path)); err != nil {
			plan.Items[i].Status, plan.Items[i].Detail = Failed, err.Error()
		}
	}
	return plan, nil
}

func repair(path string, item templates.Item, data templates.Data, earlier []string) error {
	switch item.Strategy {
	case templates.Block:
		return repairBlock(path, item, data, earlier)
	case templates.Create:
		content, err := templates.Render(item.Template, data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, content, 0o644)
	case templates.Generated:
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, item.Content, 0o644)
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

// earlierBlocks names the blocks the manifest lists before this one in the
// same file, so a missing block lands after them rather than at the top.
func earlierBlocks(items []templates.Item, path string) []string {
	var names []string
	for _, item := range items {
		if item.Strategy == templates.Block && item.Path == path {
			names = append(names, item.Block)
		}
	}
	return names
}
