package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
)

func TestRunStartURL_PrintsLocalURL(t *testing.T) {
	origLoadStackConfig := startURLLoadStackConfig
	origRemote := startURLRemote
	t.Cleanup(func() {
		startURLLoadStackConfig = origLoadStackConfig
		startURLRemote = origRemote
	})

	startURLRemote = false
	startURLLoadStackConfig = func() (*stack.StackConfig, error) {
		cfg := stack.DefaultConfig()
		cfg.Services["opencode"].Settings["host"] = "0.0.0.0"
		cfg.Services["opencode"].Settings["port"] = "7777"
		return cfg, nil
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	if err := runStartURL(cmd, nil); err != nil {
		t.Fatalf("runStartURL() returned error: %v", err)
	}

	if got := stdout.String(); got != "http://0.0.0.0:7777\n" {
		t.Fatalf("stdout = %q, want %q", got, "http://0.0.0.0:7777\n")
	}
}

func TestRunStartURL_PrintsRemoteURL(t *testing.T) {
	origLoadStackConfig := startURLLoadStackConfig
	origRemoteOpenCodeURL := startURLRemoteOpenCodeURL
	origRemote := startURLRemote
	t.Cleanup(func() {
		startURLLoadStackConfig = origLoadStackConfig
		startURLRemoteOpenCodeURL = origRemoteOpenCodeURL
		startURLRemote = origRemote
	})

	startURLRemote = true
	startURLLoadStackConfig = func() (*stack.StackConfig, error) {
		return stack.DefaultConfig(), nil
	}
	startURLRemoteOpenCodeURL = func(*stack.StackConfig) (string, error) {
		return "https://opencode.tailnet.ts.net", nil
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)

	if err := runStartURL(cmd, nil); err != nil {
		t.Fatalf("runStartURL() returned error: %v", err)
	}

	if got := stdout.String(); got != "https://opencode.tailnet.ts.net\n" {
		t.Fatalf("stdout = %q, want %q", got, "https://opencode.tailnet.ts.net\n")
	}
}

func TestRunStartURL_ReturnsCommandErrorWhenRemoteLookupFails(t *testing.T) {
	origLoadStackConfig := startURLLoadStackConfig
	origRemoteOpenCodeURL := startURLRemoteOpenCodeURL
	origRemote := startURLRemote
	t.Cleanup(func() {
		startURLLoadStackConfig = origLoadStackConfig
		startURLRemoteOpenCodeURL = origRemoteOpenCodeURL
		startURLRemote = origRemote
	})

	startURLRemote = true
	startURLLoadStackConfig = func() (*stack.StackConfig, error) {
		return stack.DefaultConfig(), nil
	}
	startURLRemoteOpenCodeURL = func(*stack.StackConfig) (string, error) {
		return "", errors.New("tailscale unavailable")
	}

	err := runStartURL(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "[error] start url: failed to resolve remote URL") {
		t.Fatalf("error missing command context: %v", err)
	}
	if !strings.Contains(err.Error(), "tailscale unavailable") {
		t.Fatalf("error missing underlying cause: %v", err)
	}
}
