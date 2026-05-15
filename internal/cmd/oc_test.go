package cmd

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestRunCode_EnsuresStackAndAttachesCurrentDirectory(t *testing.T) {
	origLoadStackConfig := ocLoadStackConfig
	origStartStack := ocStartStack
	origResolveOpenCodeBinary := ocResolveOpenCodeBinary
	origGetwd := ocGetwd
	origAttachDirectory := ocAttachDirectory
	t.Cleanup(func() {
		ocLoadStackConfig = origLoadStackConfig
		ocStartStack = origStartStack
		ocResolveOpenCodeBinary = origResolveOpenCodeBinary
		ocGetwd = origGetwd
		ocAttachDirectory = origAttachDirectory
	})

	cfg := stack.DefaultConfig()
	startCalled := false
	attachCalled := false

	ocLoadStackConfig = func() (*stack.StackConfig, error) {
		return cfg, nil
	}
	ocStartStack = func(gotCfg *stack.StackConfig, opts ...tmux.Option) (*stack.StartResult, error) {
		startCalled = true
		if gotCfg != cfg {
			t.Fatalf("startStack cfg mismatch: got %p want %p", gotCfg, cfg)
		}
		return &stack.StartResult{}, nil
	}
	ocResolveOpenCodeBinary = func() string {
		return "/usr/bin/opencode"
	}
	ocGetwd = func() (string, error) {
		return "/tmp/project", nil
	}
	ocAttachDirectory = func(binary, url, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
		attachCalled = true
		if binary != "/usr/bin/opencode" {
			t.Fatalf("binary = %q, want %q", binary, "/usr/bin/opencode")
		}
		if url != "http://127.0.0.1:4096" {
			t.Fatalf("url = %q, want %q", url, "http://127.0.0.1:4096")
		}
		if dir != "/tmp/project" {
			t.Fatalf("dir = %q, want %q", dir, "/tmp/project")
		}
		return nil
	}

	cmd := &cobra.Command{}
	if err := runCode(cmd, nil); err != nil {
		t.Fatalf("runCode() returned error: %v", err)
	}
	if !startCalled {
		t.Fatal("expected stack start to be called")
	}
	if !attachCalled {
		t.Fatal("expected attach to be called")
	}
}

func TestRunCode_ReturnsErrorWhenEnsureFails(t *testing.T) {
	origLoadStackConfig := ocLoadStackConfig
	origStartStack := ocStartStack
	origAttachDirectory := ocAttachDirectory
	t.Cleanup(func() {
		ocLoadStackConfig = origLoadStackConfig
		ocStartStack = origStartStack
		ocAttachDirectory = origAttachDirectory
	})

	attachCalled := false
	ocLoadStackConfig = func() (*stack.StackConfig, error) {
		return stack.DefaultConfig(), nil
	}
	ocStartStack = func(*stack.StackConfig, ...tmux.Option) (*stack.StartResult, error) {
		return nil, errors.New("boom")
	}
	ocAttachDirectory = func(binary, url, dir string, stdin io.Reader, stdout, stderr io.Writer) error {
		attachCalled = true
		return nil
	}

	err := runCode(&cobra.Command{}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "[error] code: failed to ensure stack. boom" {
		t.Fatalf("unexpected error: %v", err)
	}
	if attachCalled {
		t.Fatal("expected attach to be skipped when stack ensure fails")
	}
}

func TestInspectOCShimDetectsManagedStaleShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := ocShimPath()
	if err != nil {
		t.Fatalf("ocShimPath() returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	stale := "#!/bin/sh\n" + ocShimMarker + "\nexec overlord old-oc \"$@\"\n"
	if err := os.WriteFile(path, []byte(stale), 0o755); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	state, err := inspectOCShim()
	if err != nil {
		t.Fatalf("inspectOCShim() returned error: %v", err)
	}
	if state.Status != ocShimStale {
		t.Fatalf("status = %q, want %q", state.Status, ocShimStale)
	}
}

func TestEnsureOCShimInstalled_InstallsMissingShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	result, err := ensureOCShimInstalled()
	if err != nil {
		t.Fatalf("ensureOCShimInstalled() returned error: %v", err)
	}
	if result.Action != ocShimInstalled {
		t.Fatalf("action = %q, want %q", result.Action, ocShimInstalled)
	}
	if result.Status != ocShimCurrent {
		t.Fatalf("status = %q, want %q", result.Status, ocShimCurrent)
	}

	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(data) != ocShimContent() {
		t.Fatalf("unexpected shim content: %q", string(data))
	}
}

func TestEnsureOCShimInstalled_SkipsUnmanagedShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path, err := ocShimPath()
	if err != nil {
		t.Fatalf("ocShimPath() returned error: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() returned error: %v", err)
	}
	original := "#!/bin/sh\nexec custom-oc \"$@\"\n"
	if err := os.WriteFile(path, []byte(original), 0o755); err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}

	result, err := ensureOCShimInstalled()
	if err != nil {
		t.Fatalf("ensureOCShimInstalled() returned error: %v", err)
	}
	if result.Action != ocShimSkipped {
		t.Fatalf("action = %q, want %q", result.Action, ocShimSkipped)
	}
	if result.Status != ocShimConflict {
		t.Fatalf("status = %q, want %q", result.Status, ocShimConflict)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() returned error: %v", err)
	}
	if string(data) != original {
		t.Fatalf("shim content changed unexpectedly: %q", string(data))
	}
}
