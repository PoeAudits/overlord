package machine

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PoeAudits/overlord/internal/catalog"
	"github.com/PoeAudits/overlord/internal/harness"
	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/t3"
)

// Finding is one piece of drift doctor found at a path.
type Finding struct {
	Path    string
	Problem string
}

// Diagnosis is what doctor found. Notes are checks it could not complete;
// they are not drift.
type Diagnosis struct {
	Findings []Finding
	Notes    []Finding
}

// Doctor reports drift between agentsDir and every Harness under sys.Home,
// and in T3 Code's settings. It never writes.
func Doctor(ctx context.Context, sys system.System, agentsDir string) (Diagnosis, error) {
	report, err := Sync(sys, agentsDir, true)
	if err != nil {
		return Diagnosis{}, err
	}
	var diagnosis Diagnosis
	for _, result := range report.Results {
		// Hand-written agent files are found by findHandWrittenAgents, along
		// with those no Role names.
		if result.Action == OK || result.Step == "roles" && result.Action == Conflict {
			continue
		}
		diagnosis.Findings = append(diagnosis.Findings, Finding{Path: result.Path, Problem: syncProblem(result)})
	}
	dangling, err := findDanglingLinks(sys.Home, diagnosis.Findings)
	if err != nil {
		return Diagnosis{}, err
	}
	diagnosis.Findings = append(diagnosis.Findings, dangling...)
	diagnosis.Findings = append(diagnosis.Findings, findStaleSkillCopies(sys.Home, agentsDir)...)
	agents, err := findHandWrittenAgents(sys.Home)
	if err != nil {
		return Diagnosis{}, err
	}
	diagnosis.Findings = append(diagnosis.Findings, agents...)
	overrides, err := findT3OverridesWithoutSetup(ctx, sys)
	if err != nil {
		return Diagnosis{}, err
	}
	diagnosis.Findings = append(diagnosis.Findings, overrides...)
	moved, unreachable, err := findMovedUpstreams(ctx, sys.Runner, agentsDir)
	if err != nil {
		return Diagnosis{}, err
	}
	diagnosis.Findings = append(diagnosis.Findings, moved...)
	diagnosis.Notes = unreachable
	return diagnosis, nil
}

// findT3OverridesWithoutSetup names each T3 project whose scripts override
// replaces the global list without keeping the setup script. A project T3's
// database does not list, or an unreadable database, leaves only its id.
func findT3OverridesWithoutSetup(ctx context.Context, sys system.System) ([]Finding, error) {
	ids, err := t3.ProjectsWithoutSetupScript(sys.Home)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	names := map[string]string{}
	projects, _ := t3.Projects(ctx, sys.Runner, sys.Home)
	for _, project := range projects {
		names[project.ID] = fmt.Sprintf("%s (%s)", project.Title, project.Root)
	}
	var findings []Finding
	for _, id := range ids {
		name := cmp.Or(names[id], id)
		findings = append(findings, Finding{
			Path:    t3.SettingsPath(sys.Home),
			Problem: fmt.Sprintf("T3 project %s has a scripts override without %s, so Setup does not run there", name, t3.SetupScriptID),
		})
	}
	return findings, nil
}

// findMovedUpstreams compares each UPSTREAM.md entry with its remote's HEAD.
// A remote that cannot be reached, as when offline, is returned as a note.
func findMovedUpstreams(ctx context.Context, runner system.Runner, agentsDir string) (moved, unreachable []Finding, err error) {
	entries, err := catalog.LoadUpstream(agentsDir)
	if err != nil {
		return nil, nil, err
	}
	path := catalog.UpstreamPath(agentsDir)
	// Many skills come from one repository; ask each remote once.
	heads := map[string]string{}
	for _, entry := range entries {
		head, asked := heads[entry.URL]
		if !asked {
			// A private remote must fail instead of waiting on a password
			// prompt nobody sees.
			output, err := runner.Run(ctx, "", "env", "GIT_TERMINAL_PROMPT=0", "git", "ls-remote", entry.URL, "HEAD")
			if err == nil {
				head, _, _ = strings.Cut(string(output), "\t")
			}
			heads[entry.URL] = head
		}
		if head == "" {
			unreachable = append(unreachable, Finding{Path: path, Problem: fmt.Sprintf("%s: could not reach %s", entry.Name, entry.URL)})
			continue
		}
		if !strings.HasPrefix(head, entry.SHA) {
			moved = append(moved, Finding{Path: path, Problem: fmt.Sprintf("%s: %s HEAD is %.12s, recorded %s", entry.Name, entry.URL, head, entry.SHA)})
		}
	}
	return moved, unreachable, nil
}

// findDanglingLinks reports global instruction links and skill links whose
// target is gone, except paths already found.
func findDanglingLinks(home string, found []Finding) ([]Finding, error) {
	paths := harness.InstructionPaths(home)
	linksDir := harness.SkillsDir(home)
	entries, err := os.ReadDir(linksDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, entry := range entries {
		if !protectedSkillEntries[entry.Name()] {
			paths = append(paths, filepath.Join(linksDir, entry.Name()))
		}
	}
	var findings []Finding
	for _, path := range paths {
		target, err := os.Readlink(path)
		if err != nil || slices.ContainsFunc(found, func(f Finding) bool { return f.Path == path }) {
			continue
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			findings = append(findings, Finding{Path: path, Problem: "dangling link to " + DisplayPath(home, target)})
		}
	}
	return findings, nil
}

// findStaleSkillCopies reports skill directories left from before the agents
// directory became a git checkout that every Harness reads through links.
func findStaleSkillCopies(home, agentsDir string) []Finding {
	var findings []Finding
	opencode := harness.OpenCodeSkillsDir(home)
	if _, err := os.Lstat(opencode); err == nil {
		findings = append(findings, Finding{Path: opencode, Problem: "stale skill copy; OpenCode reads ~/.claude/skills, so remove it"})
	}
	_, skillsErr := os.Stat(catalog.SkillsDir(agentsDir))
	_, gitErr := os.Stat(filepath.Join(agentsDir, ".git"))
	if skillsErr == nil && errors.Is(gitErr, os.ErrNotExist) {
		findings = append(findings, Finding{Path: catalog.SkillsDir(agentsDir), Problem: "old skill copy; the agents directory is not a git checkout"})
	}
	return findings
}

// findHandWrittenAgents lists every global agent file without the header
// Sync writes, whether or not a Role has its name.
func findHandWrittenAgents(home string) ([]Finding, error) {
	var findings []Finding
	for _, h := range []harness.Harness{harness.Claude, harness.Codex, harness.OpenCode} {
		dir := harness.GlobalAgentDir(h, home)
		entries, err := os.ReadDir(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range entries {
			if !entry.Type().IsRegular() {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			if !harness.IsGenerated(content) {
				findings = append(findings, Finding{Path: path, Problem: "agent file without the generated header"})
			}
		}
	}
	return findings, nil
}

// syncProblem describes, as drift, what a Sync check would change at a path.
func syncProblem(result Result) string {
	if result.Detail == missingInstructions {
		return result.Detail
	}
	if result.Step == "instructions" {
		return instructionsProblem[result.Action]
	}
	thing := result.Detail
	if thing == "" {
		thing = syncedThing[result.Step]
	}
	switch result.Action {
	case Create:
		return thing + " missing"
	case Remove:
		return "dangling " + thing
	case Update:
		// Hooks are only ever added; the main Model can differ.
		if result.Step == "hooks" {
			return thing + " missing"
		}
		return thing + " differs from the catalog"
	case Replace:
		return thing + " out of date"
	}
	return result.Detail
}

// syncedThing names what a step puts at a path when its result has no detail.
var syncedThing = map[string]string{
	"skills": "skill link",
	"roles":  "agent file",
	"hooks":  "session sync plugin",
}

var instructionsProblem = map[Action]string{
	Create:   "global instructions link missing",
	Replace:  "not a symlink to the catalog's AGENTS.md (same content; sync replaces it)",
	Conflict: "not a symlink to the catalog's AGENTS.md (content differs; merge it by hand)",
}
