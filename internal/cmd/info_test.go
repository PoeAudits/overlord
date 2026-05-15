package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestRunInfo_JSONWritesResolvedProject(t *testing.T) {
	origLoadRegistry := infoLoadRegistry
	origJSON := infoOpts.json
	t.Cleanup(func() {
		infoLoadRegistry = origLoadRegistry
		infoOpts.json = origJSON
	})

	infoOpts.json = true
	infoLoadRegistry = func() (*registry.Registry, error) {
		return testRegistry(map[string]registry.Project{
			"dispatch": testProject("core/agents/dispatch", registry.CategoryCoreAgents, registry.LanguageGo, registry.StateActive, "ship"),
		}), nil
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	if err := runInfo(cmd, []string{"ship"}); err != nil {
		t.Fatalf("runInfo() returned error: %v", err)
	}

	var got infoOutput
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	if got.Name != "dispatch" {
		t.Fatalf("Name = %q, want %q", got.Name, "dispatch")
	}
	if got.Path != "core/agents/dispatch" {
		t.Fatalf("Path = %q, want %q", got.Path, "core/agents/dispatch")
	}
	if got.Status != registry.StateActive {
		t.Fatalf("Status = %q, want %q", got.Status, registry.StateActive)
	}
	if len(got.Aliases) != 1 || got.Aliases[0] != "ship" {
		t.Fatalf("Aliases = %v, want [ship]", got.Aliases)
	}
}

func TestRunInfo_NoMatchIncludesHint(t *testing.T) {
	origLoadRegistry := infoLoadRegistry
	origJSON := infoOpts.json
	t.Cleanup(func() {
		infoLoadRegistry = origLoadRegistry
		infoOpts.json = origJSON
	})

	infoOpts.json = false
	infoLoadRegistry = func() (*registry.Registry, error) {
		return testRegistry(map[string]registry.Project{
			"dispatch": testProject("core/agents/dispatch", registry.CategoryCoreAgents, registry.LanguageGo, registry.StateActive),
		}), nil
	}

	err := runInfo(&cobra.Command{}, []string{"missing"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "no project found matching 'missing'") {
		t.Fatalf("error missing no-match message: %v", err)
	}
	if !strings.Contains(err.Error(), "hint: run 'overlord list' to see available projects") {
		t.Fatalf("error missing hint: %v", err)
	}
}
