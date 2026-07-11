package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestRunStartRestart_RestartsServices(t *testing.T) {
	origLoadStackConfig := startRestartLoadStackConfig
	origRestartServices := startRestartServices
	t.Cleanup(func() {
		startRestartLoadStackConfig = origLoadStackConfig
		startRestartServices = origRestartServices
	})

	startRestartLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{Services: map[string]stack.ServiceConfig{}}, nil
	}

	var gotNames []string
	startRestartServices = func(cfg *stack.StackConfig, names []string, opts ...tmux.Option) (*stack.StartResult, error) {
		gotNames = append([]string(nil), names...)
		return &stack.StartResult{
			ServiceResults: []stack.ServiceResult{{Name: "opencode", DisplayName: "OpenCode", Status: stack.StatusRestarted}},
			Warnings:       []string{"split pane for ollama: failed"},
		}, nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := runStartRestart(cmd, []string{"opencode"}); err != nil {
		t.Fatalf("runStartRestart() returned error: %v", err)
	}

	if strings.Join(gotNames, ",") != "opencode" {
		t.Fatalf("restart names = %#v, want opencode", gotNames)
	}
	out := stderr.String()
	for _, want := range []string{"Restarted:", "OpenCode", "Warning:", "split pane for ollama"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
