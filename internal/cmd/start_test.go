package cmd

import (
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestDoStartSilentSkipsRegistryAndProjectSessions(t *testing.T) {
	origLoadStackConfig := startLoadStackConfig
	origLoadRegistry := startLoadRegistry
	origStartStack := startStack
	origEnsureProject := startEnsureProject
	origOpenProjectSession := startOpenProjectSession
	t.Cleanup(func() {
		startLoadStackConfig = origLoadStackConfig
		startLoadRegistry = origLoadRegistry
		startStack = origStartStack
		startEnsureProject = origEnsureProject
		startOpenProjectSession = origOpenProjectSession
	})

	loadRegistryCalled := false
	ensureProjectCalled := false
	openProjectCalled := false
	startStackCalled := false

	startLoadStackConfig = func() (*stack.StackConfig, error) {
		return &stack.StackConfig{
			Projects: []string{"chess"},
			Services: map[string]stack.ServiceConfig{},
		}, nil
	}

	startLoadRegistry = func() (*registry.Registry, error) {
		loadRegistryCalled = true
		return nil, nil
	}

	startStack = func(cfg *stack.StackConfig, opts ...tmux.Option) (*stack.StartResult, error) {
		startStackCalled = true
		return &stack.StartResult{}, nil
	}

	startEnsureProject = func(name string, project registry.Project, reg *registry.Registry) error {
		ensureProjectCalled = true
		return nil
	}

	startOpenProjectSession = func(name string, project registry.Project, reg *registry.Registry) error {
		openProjectCalled = true
		return nil
	}

	if err := doStart(true); err != nil {
		t.Fatalf("doStart(true) returned error: %v", err)
	}

	if !startStackCalled {
		t.Fatal("expected stack start to be called")
	}
	if loadRegistryCalled {
		t.Fatal("expected registry load to be skipped in silent mode")
	}
	if ensureProjectCalled {
		t.Fatal("expected project session creation to be skipped in silent mode")
	}
	if openProjectCalled {
		t.Fatal("expected project attach/open to be skipped in silent mode")
	}
}
