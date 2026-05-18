package cmd

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

func TestRunStartPair_PrintsRemotePairingOutput(t *testing.T) {
	origLoadStackConfig := startPairLoadStackConfig
	origRemoteT3CodeURL := startPairRemoteT3CodeURL
	origRunT3CLI := startPairRunT3CLI
	t.Cleanup(func() {
		startPairLoadStackConfig = origLoadStackConfig
		startPairRemoteT3CodeURL = origRemoteT3CodeURL
		startPairRunT3CLI = origRunT3CLI
	})

	cfg := stack.DefaultConfig()
	cfg.Services["t3code"].Settings["workdir"] = "/tmp/t3-code"
	cfg.Services["t3code"].Settings["base_dir"] = "/tmp/t3-state"
	startPairLoadStackConfig = func() (*stack.StackConfig, error) { return cfg, nil }
	startPairRemoteT3CodeURL = func(*stack.StackConfig) (string, error) {
		return "https://t3.tailnet.ts.net", nil
	}

	var gotWorkDir string
	var gotArgs []string
	startPairRunT3CLI = func(workDir string, args []string) ([]byte, error) {
		gotWorkDir = workDir
		gotArgs = args
		return []byte("T3 phone pairing\n"), nil
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	if err := runStartPair(cmd, nil); err != nil {
		t.Fatalf("runStartPair() returned error: %v", err)
	}

	if got := stdout.String(); got != "T3 phone pairing\n" {
		t.Fatalf("stdout = %q", got)
	}
	if gotWorkDir != "/tmp/t3-code" {
		t.Fatalf("workDir = %q", gotWorkDir)
	}
	wantArgs := []string{"apps/server/dist/bin.mjs", "auth", "pairing", "create", "--base-dir", "/tmp/t3-state", "--base-url", "https://t3.tailnet.ts.net", "--headless-output"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestRunStartPair_FallsBackToLocalURL(t *testing.T) {
	origLoadStackConfig := startPairLoadStackConfig
	origRemoteT3CodeURL := startPairRemoteT3CodeURL
	origRunT3CLI := startPairRunT3CLI
	t.Cleanup(func() {
		startPairLoadStackConfig = origLoadStackConfig
		startPairRemoteT3CodeURL = origRemoteT3CodeURL
		startPairRunT3CLI = origRunT3CLI
	})

	cfg := stack.DefaultConfig()
	startPairLoadStackConfig = func() (*stack.StackConfig, error) { return cfg, nil }
	startPairRemoteT3CodeURL = func(*stack.StackConfig) (string, error) {
		return "", errors.New("tailscale unavailable")
	}
	startPairRunT3CLI = func(workDir string, args []string) ([]byte, error) {
		if args[7] != "http://127.0.0.1:3773" {
			t.Fatalf("base URL arg = %q", args[7])
		}
		return []byte("local pairing\n"), nil
	}

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)

	if err := runStartPair(cmd, nil); err != nil {
		t.Fatalf("runStartPair() returned error: %v", err)
	}
	if !strings.Contains(stderr.String(), "failed to resolve remote Tailscale URL") {
		t.Fatalf("stderr missing warning: %q", stderr.String())
	}
}
