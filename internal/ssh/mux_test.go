package ssh

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewMux(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	if mux.controlDir == "" {
		t.Error("NewMux() controlDir is empty")
	}

	// Verify directory exists
	info, err := os.Stat(mux.controlDir)
	if err != nil {
		t.Fatalf("controlDir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("controlDir is not a directory")
	}
}

func TestMux_ControlPath(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	cp := mux.ControlPath()

	// Should be inside the control directory
	if !strings.HasPrefix(cp, mux.controlDir) {
		t.Errorf("ControlPath() = %q, does not start with controlDir %q", cp, mux.controlDir)
	}

	// Should contain the SSH template tokens
	if !strings.Contains(cp, "%r@%h:%p") {
		t.Errorf("ControlPath() = %q, does not contain %%r@%%h:%%p template", cp)
	}
}

func TestMux_Wrap(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	args := mux.Wrap()

	if len(args) != 4 {
		t.Fatalf("Wrap() returned %d args, want 4", len(args))
	}

	// Check ControlMaster=auto
	if args[0] != "-o" || args[1] != "ControlMaster=auto" {
		t.Errorf("Wrap()[0:2] = %v, want [-o ControlMaster=auto]", args[0:2])
	}

	// Check ControlPath
	if args[2] != "-o" || !strings.HasPrefix(args[3], "ControlPath=") {
		t.Errorf("Wrap()[2:4] = %v, want [-o ControlPath=...]", args[2:4])
	}

	// ControlPath should match ControlPath()
	expectedControlPath := "ControlPath=" + mux.ControlPath()
	if args[3] != expectedControlPath {
		t.Errorf("Wrap() ControlPath = %q, want %q", args[3], expectedControlPath)
	}
}

func TestMux_SSHCommand(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	cmd := mux.SSHCommand()

	// Should start with "ssh"
	if !strings.HasPrefix(cmd, "ssh ") {
		t.Errorf("SSHCommand() = %q, does not start with 'ssh '", cmd)
	}

	// Should contain ControlMaster=auto
	if !strings.Contains(cmd, "ControlMaster=auto") {
		t.Errorf("SSHCommand() = %q, does not contain ControlMaster=auto", cmd)
	}

	// Should contain the ControlPath
	if !strings.Contains(cmd, "ControlPath="+mux.ControlPath()) {
		t.Errorf("SSHCommand() = %q, does not contain ControlPath", cmd)
	}
}

func TestMux_GitSSHEnv(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	env := mux.GitSSHEnv()

	// Should start with GIT_SSH_COMMAND=
	if !strings.HasPrefix(env, "GIT_SSH_COMMAND=") {
		t.Errorf("GitSSHEnv() = %q, does not start with GIT_SSH_COMMAND=", env)
	}

	// Should contain the SSH command
	sshCmd := mux.SSHCommand()
	if !strings.Contains(env, sshCmd) {
		t.Errorf("GitSSHEnv() = %q, does not contain SSHCommand %q", env, sshCmd)
	}
}

func TestMux_Close(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}

	dir := mux.controlDir

	// Close should remove the directory
	if err := mux.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Directory should be removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Close() did not remove controlDir, err = %v", err)
	}
}

func TestMux_Close_TracksHosts(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}

	// Manually add hosts (without actually connecting - we don't have SSH targets in tests)
	mux.hosts = append(mux.hosts, "test-host-1", "test-host-2")

	// Close should not panic even with tracked hosts that have no actual connections
	if err := mux.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestMux_Close_EmptyHosts(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}

	// No hosts tracked - Close should still work
	if err := mux.Close(context.Background()); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

func TestMux_ControlDir_HasCorrectPrefix(t *testing.T) {
	t.Parallel()

	mux, err := NewMux()
	if err != nil {
		t.Fatalf("NewMux() error = %v", err)
	}
	defer mux.Close(context.Background())

	dirName := filepath.Base(mux.controlDir)
	if !strings.HasPrefix(dirName, "overlord-ssh-") {
		t.Errorf("controlDir basename = %q, does not have overlord-ssh- prefix", dirName)
	}
}
