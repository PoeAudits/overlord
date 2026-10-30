// Package system holds the two seams through which Overlord touches the
// machine: a filesystem rooted at a configurable HOME and a command runner.
// Commands receive a System instead of reading globals, so tests can supply a
// temporary HOME, a fixed environment and a fake runner.
package system

import (
	"context"
	"io"
	"os"
	"os/exec"
)

// System is everything a command may read from or do to the machine.
type System struct {
	// Home replaces the user's home directory for every path Overlord derives.
	Home    string
	Getenv  func(string) string
	Workdir string
	Runner  Runner
	Stdout  io.Writer
	Stderr  io.Writer
}

// Runner executes an external program in dir and returns its combined output.
type Runner interface {
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
}

// OS returns the System backed by the real process environment.
func OS() (System, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return System{}, err
	}
	workdir, err := os.Getwd()
	if err != nil {
		return System{}, err
	}
	return System{
		Home:    home,
		Getenv:  os.Getenv,
		Workdir: workdir,
		Runner:  ExecRunner{},
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}, nil
}

// ExecRunner runs programs with os/exec.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Dir = dir
	return command.CombinedOutput()
}
