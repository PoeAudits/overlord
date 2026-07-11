package tmux

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrSessionExists   = errors.New("tmux session already exists")
	ErrSessionNotFound = errors.New("tmux session not found")
)

// CommandRunner executes system commands. Assign a custom function for testing.
type CommandRunner func(name string, args ...string) ([]byte, error)

// Option applies configuration to a Session.
type Option func(*Session)

// Session models a tmux session and exposes helpers for interacting with it.
type Session struct {
	Name   string
	Dir    string
	Runner CommandRunner
}

// NewSession constructs a Session with optional functional modifiers.
func NewSession(name, dir string, opts ...Option) *Session {
	s := &Session{Name: name, Dir: dir}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// WithRunner injects a custom CommandRunner, useful for tests.
func WithRunner(runner CommandRunner) Option {
	return func(s *Session) {
		s.Runner = runner
	}
}

// Exists reports whether the tmux session already exists.
func (s *Session) Exists() (bool, error) {
	_, err := s.runner()("tmux", "has-session", "-t", s.Name)
	if err == nil {
		return true, nil
	}
	if code, ok := exitCode(err); ok && code == 1 {
		return false, nil
	}
	return false, fmt.Errorf("failed to check tmux session %q: %w", s.Name, err)
}

// Create starts a detached tmux session with an initial "base" window.
func (s *Session) Create() error {
	exists, err := s.Exists()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tmux session %q: %w", s.Name, ErrSessionExists)
	}
	output, err := s.runner()("tmux", "new-session", "-d", "-s", s.Name, "-n", "base", "-c", s.Dir)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// CreateWithCommand starts a detached tmux session with a named initial window running command.
func (s *Session) CreateWithCommand(windowName, command string) error {
	exists, err := s.Exists()
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("tmux session %q: %w", s.Name, ErrSessionExists)
	}
	output, err := s.runner()("tmux", "new-session", "-d", "-s", s.Name, "-n", windowName, "-c", s.Dir, command)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// NewWindow adds a new named window running command to an existing session.
func (s *Session) NewWindow(name, command string) error {
	exists, err := s.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("tmux session %q: %w", s.Name, ErrSessionNotFound)
	}
	output, err := s.runner()("tmux", "new-window", "-t", s.Name, "-n", name, "-c", s.Dir, command)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// SourceLocal executes the optional .tmux.local script with session context.
func (s *Session) SourceLocal() error {
	scriptPath := filepath.Join(s.Dir, ".tmux.local")
	if _, err := os.Stat(scriptPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat .tmux.local: %w", err)
	}
	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TMUX_SESSION=%s", s.Name),
		fmt.Sprintf("TMUX_PROJECT_DIR=%s", s.Dir),
	)
	cmd.Dir = s.Dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed != "" {
			return fmt.Errorf("failed to source .tmux.local: %s", trimmed)
		}
		return fmt.Errorf("failed to source .tmux.local: %w", err)
	}
	return nil
}

// SplitWindow creates a vertical split in the named window, running command in the new pane.
func (s *Session) SplitWindow(windowName, command string) error {
	target := s.Name + ":" + windowName
	args := []string{"split-window", "-v", "-t", target, "-c", s.Dir, command}
	output, err := s.runner()("tmux", args...)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// KillWindow terminates a named tmux window in the session.
func (s *Session) KillWindow(windowName string) error {
	target := s.Name + ":" + windowName
	output, err := s.runner()("tmux", "kill-window", "-t", target)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// IsWindowNotFound reports whether err signals that tmux could not find the
// targeted window. It matches the literal "can't find window: <name>" output
// tmux emits when kill-window targets a missing window in an existing
// session. Callers that want to treat a missing window as a soft failure
// (e.g. "ensure this window exists") should consult this helper instead of
// blanket-ignoring kill-window errors.
func IsWindowNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "can't find window:")
}

// Kill terminates the tmux session if it exists.
func (s *Session) Kill() error {
	exists, err := s.Exists()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	output, err := s.runner()("tmux", "kill-session", "-t", s.Name)
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return errors.New(trimmed)
		}
		return err
	}
	return nil
}

// AttachOrSwitch either attaches to the session or switches the current client.
func (s *Session) AttachOrSwitch() error {
	args := []string{"attach", "-t", s.Name}
	insideTmux := os.Getenv("TMUX") != ""
	if insideTmux {
		args = []string{"switch-client", "-t", s.Name}
	}
	_, err := s.runner()("tmux", args...)
	if err != nil {
		if code, ok := exitCode(err); ok && code == 1 {
			return fmt.Errorf("tmux session %q missing: %w", s.Name, ErrSessionNotFound)
		}
		return fmt.Errorf("failed to run tmux %s: %w", args[0], err)
	}
	return nil
}

func (s *Session) runner() CommandRunner {
	if s.Runner == nil {
		s.Runner = defaultRunner
	}
	return s.Runner
}

func defaultRunner(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if name == "tmux" && len(args) > 0 {
		switch args[0] {
		case "attach", "switch-client":
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			return nil, cmd.Run()
		}
	}
	output, err := cmd.CombinedOutput()
	return output, err
}

func exitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	var coder interface {
		ExitCode() int
	}
	if errors.As(err, &coder) {
		return coder.ExitCode(), true
	}
	return 0, false
}
