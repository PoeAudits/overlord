package cmd

import (
	"testing"

	"github.com/PoeAudits/overlord/internal/checkpoint"
	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestDoStartSilentSkipsRegistryAndProjectSessions(t *testing.T) {
	origLoadStackConfig := startLoadStackConfig
	origStartStack := startStack
	origLoadCheckpoints := startLoadCheckpoints
	origRestoreCheckpoint := startRestoreCheckpoint
	origAttach := startAttach
	t.Cleanup(func() {
		startLoadStackConfig = origLoadStackConfig
		startStack = origStartStack
		startLoadCheckpoints = origLoadCheckpoints
		startRestoreCheckpoint = origRestoreCheckpoint
		startAttach = origAttach
	})

	loadCheckpointsCalled := false
	restoreCheckpointCalled := false
	attachCalled := false
	startStackCalled := false

	startLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{Services: map[string]stack.ServiceConfig{}}, nil
	}

	startLoadCheckpoints = func() (*checkpoint.Store, error) {
		loadCheckpointsCalled = true
		return nil, nil
	}

	startStack = func(cfg *stack.StackConfig, opts ...tmux.Option) (*stack.StartResult, error) {
		startStackCalled = true
		return &stack.StartResult{}, nil
	}

	startRestoreCheckpoint = func(*checkpoint.Checkpoint, tmux.CommandRunner, func(string, ...any)) []string {
		restoreCheckpointCalled = true
		return nil
	}
	startAttach = func(string) error { attachCalled = true; return nil }

	if err := doStart(true); err != nil {
		t.Fatalf("doStart(true) returned error: %v", err)
	}

	if !startStackCalled {
		t.Fatal("expected stack start to be called")
	}
	if loadCheckpointsCalled {
		t.Fatal("expected checkpoint load to be skipped in silent mode")
	}
	if restoreCheckpointCalled {
		t.Fatal("expected workspace restoration to be skipped in silent mode")
	}
	if attachCalled {
		t.Fatal("expected tmux attach to be skipped in silent mode")
	}
}

func TestDoStartRestoresCheckpointAndAttachesSavedSession(t *testing.T) {
	origLoadStackConfig, origStartStack := startLoadStackConfig, startStack
	origLoadCheckpoints, origRestore, origAttach := startLoadCheckpoints, startRestoreCheckpoint, startAttach
	t.Cleanup(func() {
		startLoadStackConfig, startStack = origLoadStackConfig, origStartStack
		startLoadCheckpoints, startRestoreCheckpoint, startAttach = origLoadCheckpoints, origRestore, origAttach
	})
	startLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{Services: map[string]stack.ServiceConfig{}}, nil
	}
	startStack = func(*stack.StackConfig, ...tmux.Option) (*stack.StartResult, error) { return &stack.StartResult{}, nil }
	startLoadCheckpoints = func() (*checkpoint.Store, error) {
		return &checkpoint.Store{Current: &checkpoint.Checkpoint{AttachSession: "saved"}}, nil
	}
	startRestoreCheckpoint = func(*checkpoint.Checkpoint, tmux.CommandRunner, func(string, ...any)) []string {
		return []string{"other", "saved"}
	}
	var attached string
	startAttach = func(name string) error { attached = name; return nil }
	if err := doStart(false); err != nil {
		t.Fatal(err)
	}
	if attached != "saved" {
		t.Fatalf("attached %q, want saved", attached)
	}
}

func TestDoStartWithoutCheckpointAttachesStartServer(t *testing.T) {
	origLoadStackConfig, origStartStack := startLoadStackConfig, startStack
	origLoadCheckpoints, origAttach := startLoadCheckpoints, startAttach
	t.Cleanup(func() {
		startLoadStackConfig, startStack = origLoadStackConfig, origStartStack
		startLoadCheckpoints, startAttach = origLoadCheckpoints, origAttach
	})
	startLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{Services: map[string]stack.ServiceConfig{}}, nil
	}
	startStack = func(*stack.StackConfig, ...tmux.Option) (*stack.StartResult, error) { return &stack.StartResult{}, nil }
	startLoadCheckpoints = func() (*checkpoint.Store, error) { return &checkpoint.Store{}, nil }
	var attached string
	startAttach = func(name string) error { attached = name; return nil }
	if err := doStart(false); err != nil {
		t.Fatal(err)
	}
	if attached != stack.SessionName {
		t.Fatalf("attached %q, want %q", attached, stack.SessionName)
	}
}
