package project

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// repoRow checks that the Project has an origin remote. Without one, the row
// records what Apply would do with newRepo (owner/name), or is a conflict
// saying why it must not: the checkout sits inside another repository, its
// history tracks a Secret, or newRepo exists on GitHub and is not empty.
func repoRow(ctx context.Context, runner system.Runner, root, newRepo string, data templates.Data, row Item) Item {
	if data.Origin != "" {
		row.Status = OK
		return row
	}
	row.repo = newRepo
	toplevel, err := runner.Run(ctx, root, "git", "rev-parse", "--show-toplevel")
	row.gitInit = err != nil
	if !row.gitInit {
		if outer := strings.TrimSpace(string(toplevel)); outer != resolved(root) {
			row.Status, row.Detail = Conflict, "inside the git repository at "+outer+"; no GitHub repository created"
			return row
		}
		secrets, err := trackedSecrets(ctx, runner, root)
		if err != nil {
			row.Status, row.Detail = Conflict, "could not list the files in its history ("+err.Error()+"); nothing pushed"
			return row
		}
		if len(secrets) > 0 {
			row.Status, row.Detail = Conflict, "history tracks secret files: "+strings.Join(secrets, ", ")+"; nothing is pushed until they are removed from every branch's and tag's history (git filter-repo) and overlord setup re-runs"
			return row
		}
	}
	output, err := runner.Run(ctx, root, "gh", "repo", "view", newRepo, "--json", "isEmpty")
	if err != nil && strings.Contains(err.Error(), "Could not resolve to a Repository") {
		row.Status, row.Detail = Missing, "would create private "+newRepo+" and push every branch and tag"
		if row.gitInit {
			row.Detail = "would git init, create private " + newRepo + " and push every branch and tag"
		}
		return row
	}
	if err != nil {
		row.Status, row.Detail = Warning, "gh repo view "+newRepo+" failed: "+failure(output, err)+"; no GitHub repository created"
		return row
	}
	var view struct {
		IsEmpty bool `json:"isEmpty"`
	}
	if err := json.Unmarshal(output, &view); err != nil || !view.IsEmpty {
		row.Status, row.Detail = Conflict, newRepo+" already exists on GitHub and is not empty; left untouched, so add it as origin yourself or rename the directory"
		return row
	}
	row.existing = true
	row.Status, row.Detail = Missing, "would push every branch and tag to the existing empty "+newRepo
	return row
}

// createRepo does what repoRow planned: git init when needed, create the
// private repository (or add the existing empty one as origin), then push
// every branch and tag. It returns the row, failed at the first step that
// fails.
func createRepo(ctx context.Context, runner system.Runner, root string, row Item) Item {
	pushed, err := createAndPush(ctx, runner, root, row)
	if err != nil {
		row.Status, row.Detail = Failed, err.Error()
		return row
	}
	if !pushed {
		row.Detail = "private " + row.repo + " created; nothing to push yet"
		if row.existing {
			row.Detail = "the existing empty " + row.repo + " added as origin; nothing to push yet"
		}
		return row
	}
	row.Detail = "private " + row.repo + ", every branch and tag pushed"
	if row.existing {
		row.Detail = "the existing empty " + row.repo + ", every branch and tag pushed"
	}
	return row
}

// createAndPush reports whether it pushed anything; a repository without
// branches has nothing to push.
func createAndPush(ctx context.Context, runner system.Runner, root string, row Item) (bool, error) {
	if row.gitInit {
		if err := step(ctx, runner, root, "git", "init"); err != nil {
			return false, err
		}
	}
	origin := []string{"gh", "repo", "create", row.repo, "--private", "--source", root, "--remote", "origin"}
	if row.existing {
		origin = []string{"git", "remote", "add", "origin", remoteURL(ctx, runner, root, row.repo)}
	}
	if err := step(ctx, runner, root, origin[0], origin[1:]...); err != nil {
		return false, err
	}
	branches, err := runner.Run(ctx, root, "git", "for-each-ref", "--format=%(refname)", "refs/heads")
	if err != nil {
		return false, fmt.Errorf("git for-each-ref failed: %s", failure(branches, err))
	}
	// git refuses to push when there is nothing to push, as in a new repository.
	if len(strings.TrimSpace(string(branches))) == 0 {
		return false, nil
	}
	if err := step(ctx, runner, root, "git", "push", "-u", "origin", "refs/heads/*:refs/heads/*", "refs/tags/*:refs/tags/*"); err != nil {
		return false, fmt.Errorf("%w; origin now exists, so overlord setup will not push again: push every branch and tag yourself", err)
	}
	return true, nil
}

// remoteURL is the clone URL of repo in the protocol gh is configured for.
func remoteURL(ctx context.Context, runner system.Runner, root, repo string) string {
	protocol, err := runner.Run(ctx, root, "gh", "config", "get", "git_protocol")
	if err == nil && strings.TrimSpace(string(protocol)) == "ssh" {
		return "git@github.com:" + repo + ".git"
	}
	return "https://github.com/" + repo + ".git"
}

// trackedSecrets lists, sorted, every path that any branch's or tag's history
// has tracked and that matches templates.Secrets.
func trackedSecrets(ctx context.Context, runner system.Runner, root string) ([]string, error) {
	// quotePath off keeps non-ASCII paths as they are, so their base name
	// still matches.
	output, err := runner.Run(ctx, root, "git", "-c", "core.quotePath=false", "log", "--branches", "--tags", "--name-only", "--format=")
	if err != nil {
		return nil, fmt.Errorf("git log failed: %s", failure(output, err))
	}
	var found []string
	for _, path := range strings.Split(string(output), "\n") {
		if path != "" && isSecret(path) && !slices.Contains(found, path) {
			found = append(found, path)
		}
	}
	slices.Sort(found)
	return found, nil
}

// isSecret applies templates.Secrets to the path's base name the way git
// applies .gitignore lines: the last matching pattern decides.
func isSecret(path string) bool {
	secret := false
	for _, pattern := range templates.Secrets {
		glob, except := strings.CutPrefix(pattern, "!")
		if matched, _ := filepath.Match(glob, filepath.Base(path)); matched {
			secret = !except
		}
	}
	return secret
}

// resolved is path with symlinks resolved, as git prints a top level.
func resolved(path string) string {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return target
}
