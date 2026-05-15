package stack

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// mockRunner returns a CommandRunner that records calls and behaves according
// to the supplied handler. If handler is nil, all commands succeed with empty
// output.
func mockRunner(handler func(name string, args ...string) ([]byte, error)) tmux.CommandRunner {
	if handler == nil {
		handler = func(string, ...string) ([]byte, error) { return nil, nil }
	}
	return handler
}

// exitError implements the ExitCode interface expected by tmux.exitCode.
type exitError struct {
	code int
}

func (e *exitError) Error() string { return fmt.Sprintf("exit status %d", e.code) }
func (e *exitError) ExitCode() int { return e.code }

// sessionNotExistsRunner simulates "has-session" returning exit code 1
// (session does not exist) and all other commands succeeding.
func sessionNotExistsRunner() tmux.CommandRunner {
	return mockRunner(func(name string, args ...string) ([]byte, error) {
		if name == "tmux" && len(args) > 0 && args[0] == "has-session" {
			return nil, &exitError{code: 1}
		}
		return nil, nil
	})
}

// sessionExistsRunner simulates "has-session" succeeding (session exists).
func sessionExistsRunner() tmux.CommandRunner {
	return mockRunner(func(name string, args ...string) ([]byte, error) {
		return nil, nil
	})
}

func enabledConfig(names ...string) *StackConfig {
	services := make(map[string]ServiceConfig)
	for _, n := range names {
		services[n] = ServiceConfig{Enabled: true}
	}
	return &StackConfig{
		Version:  1,
		Services: services,
	}
}

// stubService implements Service for testing with controllable health checks.
type stubService struct {
	name        string
	windowName  string
	command     string
	workDir     string
	env         []string
	paneCommand string
	healthErr   error
	healthCalls atomic.Int64
}

func (s *stubService) Name() string        { return s.name }
func (s *stubService) WindowName() string  { return s.windowName }
func (s *stubService) Command() string     { return s.command }
func (s *stubService) WorkDir() string     { return s.workDir }
func (s *stubService) Env() []string       { return s.env }
func (s *stubService) PaneCommand() string { return s.paneCommand }
func (s *stubService) HealthCheck() error {
	s.healthCalls.Add(1)
	return s.healthErr
}

func TestStartStack_SessionAlreadyExists(t *testing.T) {
	cfg := enabledConfig("opencode", "frontend")

	result, err := StartStack(cfg, tmux.WithRunner(sessionExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ServiceResults) != 2 {
		t.Fatalf("expected 2 service results, got %d", len(result.ServiceResults))
	}
	for _, sr := range result.ServiceResults {
		if sr.Status != StatusSkipped {
			t.Errorf("service %s: expected status 'skipped', got %q", sr.Name, sr.Status)
		}
	}
}

func TestStartStack_SuccessfulStart(t *testing.T) {
	// Track tmux commands issued and session state.
	var commands []string
	sessionCreated := false
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		commands = append(commands, cmd)
		if name == "tmux" && len(args) > 0 {
			switch args[0] {
			case "has-session":
				if !sessionCreated {
					return nil, &exitError{code: 1}
				}
				return nil, nil
			case "new-session":
				sessionCreated = true
				return nil, nil
			}
		}
		return nil, nil
	})

	cfg := enabledConfig("opencode", "frontend")

	result, err := StartStack(cfg, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.ServiceResults) != 2 {
		t.Fatalf("expected 2 service results, got %d", len(result.ServiceResults))
	}
	for _, sr := range result.ServiceResults {
		if sr.Status != StatusStarted {
			t.Errorf("service %s: expected status 'started', got %q", sr.Name, sr.Status)
		}
	}

	// Verify new-session was called (for the first service).
	foundNewSession := false
	foundNewWindow := false
	for _, c := range commands {
		if strings.Contains(c, "new-session") {
			foundNewSession = true
		}
		if strings.Contains(c, "new-window") {
			foundNewWindow = true
		}
	}
	if !foundNewSession {
		t.Error("expected tmux new-session command")
	}
	if !foundNewWindow {
		t.Error("expected tmux new-window command for second service")
	}
}

func TestStartStack_RollsBackOnWindowCreationFailure(t *testing.T) {
	var commands []string
	sessionCreated := false
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		commands = append(commands, cmd)
		if name == "tmux" && len(args) > 0 {
			switch args[0] {
			case "has-session":
				if !sessionCreated {
					return nil, &exitError{code: 1}
				}
				return nil, nil
			case "new-session":
				sessionCreated = true
				return nil, nil
			case "new-window":
				return nil, errors.New("new-window failed")
			}
		}
		return nil, nil
	})

	cfg := enabledConfig("opencode", "frontend")
	_, err := StartStack(cfg, tmux.WithRunner(runner))
	if err == nil {
		t.Fatal("expected error")
	}

	foundKill := false
	for _, c := range commands {
		if strings.Contains(c, "kill-session") {
			foundKill = true
			break
		}
	}
	if !foundKill {
		t.Fatal("expected kill-session rollback command")
	}
}

func TestStartStack_NoEnabledServices(t *testing.T) {
	cfg := &StackConfig{
		Version:  1,
		Services: map[string]ServiceConfig{},
	}

	result, err := StartStack(cfg, tmux.WithRunner(sessionNotExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.ServiceResults) != 0 {
		t.Errorf("expected 0 service results, got %d", len(result.ServiceResults))
	}
}

func TestStartStack_PreHookFailure(t *testing.T) {
	// Register a pre-hook that fails.
	bus := lifecycle.DefaultBus
	bus.Subscribe(lifecycle.StackStarting, func(e lifecycle.Event) error {
		return errors.New("pre-hook exploded")
	})
	defer func() {
		// Reset the bus by replacing DefaultBus.
		*lifecycle.DefaultBus = *lifecycle.NewBus()
	}()

	cfg := enabledConfig("opencode")

	_, err := StartStack(cfg, tmux.WithRunner(sessionNotExistsRunner()))
	if err == nil {
		t.Fatal("expected error from pre-hook failure")
	}
	if !strings.Contains(err.Error(), "pre-hook") {
		t.Errorf("expected error to mention 'pre-hook', got: %v", err)
	}
}

func TestBuildCommand_WithEnv(t *testing.T) {
	svc := &stubService{
		command: "bun serve.ts",
		env:     []string{"PORT=3001", "API_URL=http://localhost:4096"},
	}
	got := buildCommand(svc)
	want := "env PORT=3001 API_URL=http://localhost:4096 bun serve.ts"
	if got != want {
		t.Errorf("buildCommand:\ngot:  %s\nwant: %s", got, want)
	}
}

func TestBuildCommand_NoEnv(t *testing.T) {
	svc := &stubService{
		command: "ollama serve",
	}
	got := buildCommand(svc)
	if got != "ollama serve" {
		t.Errorf("expected plain command, got: %s", got)
	}
}
