package ssh

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Mux manages SSH ControlMaster connections for connection multiplexing.
// It establishes a persistent master connection per host, so subsequent SSH
// operations (rsync, git, direct ssh) reuse the master without re-authenticating.
type Mux struct {
	controlDir string
	hosts      []string // tracked hosts for cleanup
}

// NewMux creates a new SSH multiplexer with a temp directory for control sockets.
func NewMux() (*Mux, error) {
	dir, err := os.MkdirTemp("", "overlord-ssh-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH control directory: %w", err)
	}
	return &Mux{controlDir: dir}, nil
}

// ControlPath returns the control socket path template for SSH options.
// The template uses %r@%h:%p so each unique user@host:port gets its own socket.
func (m *Mux) ControlPath() string {
	return filepath.Join(m.controlDir, "%r@%h:%p")
}

// Connect establishes a ControlMaster connection to the given host.
// This is the only operation that prompts for authentication.
// The connection backgrounds after auth and stays alive for reuse.
// host may include a user prefix (e.g. "root@10.0.0.1").
func (m *Mux) Connect(ctx context.Context, host string) error {
	args := []string{
		"-o", "ControlMaster=yes",
		"-o", "ControlPath=" + m.ControlPath(),
		"-o", "ControlPersist=10m",
		"-N", // no remote command
		"-f", // go to background after auth
		host,
	}
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to establish SSH master connection to %s: %w", host, err)
	}
	m.hosts = append(m.hosts, host)
	return nil
}

// Wrap returns SSH arguments to use the multiplexed connection.
// These should be prepended to direct SSH command arguments.
//
// Example:
//
//	args := append(mux.Wrap(), host, "mkdir", "-p", path)
//	exec.Command("ssh", args...)
func (m *Mux) Wrap() []string {
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + m.ControlPath(),
	}
}

// SSHCommand returns a full SSH command string for use with rsync -e
// or GIT_SSH_COMMAND environment variable.
func (m *Mux) SSHCommand() string {
	return fmt.Sprintf("ssh -o ControlMaster=auto -o ControlPath=%s", m.ControlPath())
}

// GitSSHEnv returns the GIT_SSH_COMMAND environment variable string
// for passing to git operations.
func (m *Mux) GitSSHEnv() string {
	return "GIT_SSH_COMMAND=" + m.SSHCommand()
}

// Close terminates all ControlMaster connections and removes the control directory.
// Errors during cleanup are ignored (best-effort).
func (m *Mux) Close(ctx context.Context) error {
	for _, host := range m.hosts {
		cmd := exec.CommandContext(ctx, "ssh",
			"-o", "ControlPath="+m.ControlPath(),
			"-O", "exit",
			host,
		)
		cmd.Run() // best effort
	}
	return os.RemoveAll(m.controlDir)
}
