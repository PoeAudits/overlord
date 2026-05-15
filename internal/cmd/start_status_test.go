package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

func TestRunStartStatus_PrintsOpenCodeDriftWhenStackStopped(t *testing.T) {
	origLoadStackConfig := startStatusLoadStackConfig
	origSessionExists := startStatusSessionExists
	origDefaultServices := startStatusDefaultServices
	origRemoteT3CodeURL := startStatusRemoteT3CodeURL
	origResolveOpenCodeBinary := startStatusResolveOpenCodeBinary
	origInspectOCShim := startStatusInspectOCShim
	t.Cleanup(func() {
		startStatusLoadStackConfig = origLoadStackConfig
		startStatusSessionExists = origSessionExists
		startStatusDefaultServices = origDefaultServices
		startStatusRemoteT3CodeURL = origRemoteT3CodeURL
		startStatusResolveOpenCodeBinary = origResolveOpenCodeBinary
		startStatusInspectOCShim = origInspectOCShim
	})

	startStatusLoadStackConfig = func() (*stack.StackConfig, error) {
		return stack.DefaultConfig(), nil
	}
	startStatusSessionExists = func() (bool, error) {
		return false, nil
	}
	startStatusDefaultServices = func(*stack.StackConfig) []stack.Service {
		return nil
	}
	startStatusRemoteT3CodeURL = func(*stack.StackConfig) (string, error) {
		return "", errors.New("missing")
	}
	startStatusResolveOpenCodeBinary = func() string {
		return "/usr/bin/opencode"
	}
	startStatusInspectOCShim = func() (ocShimState, error) {
		return ocShimState{Path: "/tmp/oc", Status: ocShimStale}, nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := runStartStatus(cmd, nil); err != nil {
		t.Fatalf("runStartStatus() returned error: %v", err)
	}

	out := stderr.String()
	for _, want := range []string{"StartServer is not running", "T3 Code URL", "OpenCode URL", "/usr/bin/opencode", "/tmp/oc", "stale"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
