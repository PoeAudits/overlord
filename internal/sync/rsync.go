package sync

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Direction represents the sync direction
type Direction string

const (
	// DirectionPush syncs from local to remote
	DirectionPush Direction = "push"
	// DirectionPull syncs from remote to local
	DirectionPull Direction = "pull"
)

// RsyncOptions configures an rsync operation
type RsyncOptions struct {
	// Source is the source path (local path for push, remote path for pull)
	Source string

	// Dest is the destination path (remote path for push, local path for pull)
	Dest string

	// Excludes is a list of patterns to exclude from sync
	Excludes []string

	// DryRun if true, performs a trial run with no changes made
	DryRun bool

	// Delete if true, deletes extraneous files from destination
	Delete bool
}

// Validate checks if the options are valid
func (o *RsyncOptions) Validate() error {
	if o.Source == "" {
		return fmt.Errorf("source path is required")
	}
	if o.Dest == "" {
		return fmt.Errorf("destination path is required")
	}
	return nil
}

// RsyncResult contains the result of an rsync operation
type RsyncResult struct {
	// Output contains the combined stdout/stderr from rsync
	Output string

	// ExitCode is the rsync exit code
	ExitCode int
}

// CommandExecutor is an interface for executing commands
// This allows for mocking in tests
type CommandExecutor interface {
	// Execute runs a command and returns the combined output and error
	Execute(name string, args ...string) (output []byte, exitCode int, err error)
}

// DefaultExecutor uses os/exec to run commands
type DefaultExecutor struct{}

// Execute runs a command using os/exec
func (e *DefaultExecutor) Execute(name string, args ...string) ([]byte, int, error) {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine stdout and stderr
	combined := stdout.String() + stderr.String()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start
			return []byte(combined), -1, fmt.Errorf("failed to execute rsync: %w", err)
		}
	}

	return []byte(combined), exitCode, nil
}

// LookPathFunc is a function type for looking up executables in PATH
type LookPathFunc func(file string) (string, error)

// Rsync wraps rsync operations
type Rsync struct {
	executor CommandExecutor
	lookPath LookPathFunc
}

// NewRsync creates a new Rsync instance with the default executor
func NewRsync() *Rsync {
	return &Rsync{
		executor: &DefaultExecutor{},
		lookPath: exec.LookPath,
	}
}

// NewRsyncWithExecutor creates a new Rsync instance with a custom executor
// This is useful for testing
func NewRsyncWithExecutor(executor CommandExecutor) *Rsync {
	return &Rsync{
		executor: executor,
		lookPath: func(file string) (string, error) {
			// In test mode, always return success for rsync lookup
			return file, nil
		},
	}
}

// buildArgs constructs the rsync command arguments
func (r *Rsync) buildArgs(opts RsyncOptions) []string {
	args := []string{
		"-avz",       // archive mode, verbose, compress
		"--progress", // show progress during transfer
	}

	if opts.DryRun {
		args = append(args, "--dry-run")
	}

	if opts.Delete {
		args = append(args, "--delete")
	}

	// Add exclusion patterns
	for _, pattern := range opts.Excludes {
		args = append(args, "--exclude="+pattern)
	}

	// Add source and destination
	// Ensure source directory ends with / for rsync to sync contents
	source := opts.Source
	if !strings.HasSuffix(source, "/") {
		source += "/"
	}

	args = append(args, source, opts.Dest)

	return args
}

// Push syncs files from local to remote (local -> remote)
// The remote path should be in the format: user@host:path or host:path
func (r *Rsync) Push(opts RsyncOptions) (*RsyncResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Check if rsync is available
	if _, err := r.lookPath("rsync"); err != nil {
		return nil, fmt.Errorf("rsync not found in PATH: %w", err)
	}

	args := r.buildArgs(opts)
	output, exitCode, err := r.executor.Execute("rsync", args...)
	if err != nil {
		return nil, err
	}

	result := &RsyncResult{
		Output:   string(output),
		ExitCode: exitCode,
	}

	if exitCode != 0 {
		return result, fmt.Errorf("rsync failed with exit code %d: %s", exitCode, strings.TrimSpace(string(output)))
	}

	return result, nil
}

// Pull syncs files from remote to local (remote -> local)
// The remote path should be in the format: user@host:path or host:path
func (r *Rsync) Pull(opts RsyncOptions) (*RsyncResult, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Check if rsync is available
	if _, err := r.lookPath("rsync"); err != nil {
		return nil, fmt.Errorf("rsync not found in PATH: %w", err)
	}

	args := r.buildArgs(opts)
	output, exitCode, err := r.executor.Execute("rsync", args...)
	if err != nil {
		return nil, err
	}

	result := &RsyncResult{
		Output:   string(output),
		ExitCode: exitCode,
	}

	if exitCode != 0 {
		return result, fmt.Errorf("rsync failed with exit code %d: %s", exitCode, strings.TrimSpace(string(output)))
	}

	return result, nil
}

// DryRun performs a dry run of the sync operation and returns what would be transferred
// This is useful for previewing changes before actually syncing
func (r *Rsync) DryRun(opts RsyncOptions) (string, error) {
	// Force dry-run mode
	opts.DryRun = true

	if err := opts.Validate(); err != nil {
		return "", fmt.Errorf("invalid options: %w", err)
	}

	// Check if rsync is available
	if _, err := r.lookPath("rsync"); err != nil {
		return "", fmt.Errorf("rsync not found in PATH: %w", err)
	}

	args := r.buildArgs(opts)
	output, exitCode, err := r.executor.Execute("rsync", args...)
	if err != nil {
		return "", err
	}

	if exitCode != 0 {
		return string(output), fmt.Errorf("rsync dry-run failed with exit code %d: %s", exitCode, strings.TrimSpace(string(output)))
	}

	return string(output), nil
}

// FormatRemotePath formats a remote path for rsync
// It combines host and path into the format expected by rsync: host:path
func FormatRemotePath(host, path string) string {
	return fmt.Sprintf("%s:%s", host, path)
}

// FormatRemotePathWithUser formats a remote path with user for rsync
// It combines user, host and path into the format: user@host:path
func FormatRemotePathWithUser(user, host, path string) string {
	return fmt.Sprintf("%s@%s:%s", user, host, path)
}

// ParseRemotePath parses a remote path in the format [user@]host:path
// Returns user (empty if not specified), host, and path
func ParseRemotePath(remotePath string) (user, host, path string, err error) {
	// Find the colon that separates host from path
	colonIdx := strings.Index(remotePath, ":")
	if colonIdx == -1 {
		return "", "", "", fmt.Errorf("invalid remote path format: missing colon separator")
	}

	hostPart := remotePath[:colonIdx]
	path = remotePath[colonIdx+1:]

	// Check for user@host format
	atIdx := strings.Index(hostPart, "@")
	if atIdx != -1 {
		user = hostPart[:atIdx]
		host = hostPart[atIdx+1:]
	} else {
		host = hostPart
	}

	if host == "" {
		return "", "", "", fmt.Errorf("invalid remote path format: empty host")
	}

	return user, host, path, nil
}

// IsRemotePath checks if a path is a remote path (contains host:path format)
func IsRemotePath(path string) bool {
	// Remote paths contain a colon but don't start with / (which would be absolute local path)
	// and the colon is not at position 1 (which would be Windows drive letter)
	colonIdx := strings.Index(path, ":")
	if colonIdx == -1 {
		return false
	}
	// Not a Windows drive letter (e.g., C:)
	if colonIdx == 1 {
		return false
	}
	// Not an absolute path that happens to contain a colon
	if strings.HasPrefix(path, "/") {
		return false
	}
	return true
}
