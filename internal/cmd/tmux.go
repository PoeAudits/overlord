package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// tmuxSession represents a tmux session configuration
type tmuxSession struct {
	Name string
	Dir  string
}

// exists checks if the tmux session already exists
func (t *tmuxSession) exists() bool {
	checkSession := exec.Command("tmux", "has-session", "-t", t.Name)
	return checkSession.Run() == nil
}

// create creates a new detached tmux session
func (t *tmuxSession) create() error {
	// Create new session (detached) with window named "base" for .tmux.local compatibility
	createSession := exec.Command("tmux", "new-session", "-d", "-s", t.Name, "-n", "base", "-c", t.Dir)
	if output, err := createSession.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create tmux session: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// sourceLocal sources the .tmux.local file if it exists
func (t *tmuxSession) sourceLocal() error {
	tmuxLocalPath := filepath.Join(t.Dir, ".tmux.local")
	if _, err := os.Stat(tmuxLocalPath); err != nil {
		// File doesn't exist - not an error
		return nil
	}

	// Run .tmux.local as a bash script with environment variables
	tmuxSource := exec.Command("bash", tmuxLocalPath)
	tmuxSource.Env = append(os.Environ(),
		fmt.Sprintf("TMUX_SESSION=%s", t.Name),
		fmt.Sprintf("TMUX_PROJECT_DIR=%s", t.Dir),
	)
	if output, err := tmuxSource.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to source .tmux.local: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

// attachOrSwitch attaches to the session or switches if already in tmux
func (t *tmuxSession) attachOrSwitch() error {
	if os.Getenv("TMUX") != "" {
		// Already in tmux, switch client
		switchCmd := exec.Command("tmux", "switch-client", "-t", t.Name)
		switchCmd.Stdin = os.Stdin
		switchCmd.Stdout = os.Stdout
		switchCmd.Stderr = os.Stderr
		if err := switchCmd.Run(); err != nil {
			return fmt.Errorf("failed to switch to session: %w", err)
		}
	} else {
		// Not in tmux, attach
		attachCmd := exec.Command("tmux", "attach", "-t", t.Name)
		attachCmd.Stdin = os.Stdin
		attachCmd.Stdout = os.Stdout
		attachCmd.Stderr = os.Stderr
		if err := attachCmd.Run(); err != nil {
			return fmt.Errorf("failed to attach to session: %w", err)
		}
	}
	return nil
}
