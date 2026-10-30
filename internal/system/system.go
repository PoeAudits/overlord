// Package system holds the seams through which Overlord touches the machine:
// a filesystem rooted at a configurable HOME, a command runner and an HTTP
// health checker. Commands receive a System instead of reading globals, so
// tests can supply a temporary HOME, a fixed environment, a fake runner and a
// fake health checker.
package system

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// System is everything a command may read from or do to the machine.
type System struct {
	// Home replaces the user's home directory for every path Overlord derives.
	Home    string
	Getenv  func(string) string
	Workdir string
	Runner  Runner
	Health  HealthChecker
	// Sleep pauses between health checks while start waits for a service.
	Sleep  func(time.Duration)
	Stdout io.Writer
	Stderr io.Writer
}

// Runner executes external programs.
type Runner interface {
	// Run executes name in dir, waits for it and returns its combined output.
	Run(ctx context.Context, dir, name string, args ...string) ([]byte, error)
	// Launch starts name in dir detached from Overlord, with stdout and
	// stderr appended to logPath, and returns without waiting for it.
	Launch(dir, logPath, name string, args ...string) error
}

// HealthChecker reports whether a service answers at a health URL.
type HealthChecker interface {
	Healthy(ctx context.Context, url string) bool
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
		Health:  HTTPHealth{},
		Sleep:   time.Sleep,
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

func (ExecRunner) Launch(dir, logPath, name string, args ...string) error {
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	// The child holds its own copy of the descriptor once started.
	defer log.Close()
	command := exec.Command(name, args...)
	command.Dir = dir
	command.Stdout = log
	command.Stderr = log
	// A new session keeps the service running after Overlord and its
	// terminal exit.
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

// HTTPHealth checks health with an HTTP GET. Any response below 500 counts as
// healthy: the service is listening and serving, even if it demands
// credentials Overlord does not have.
type HTTPHealth struct{}

func (HTTPHealth) Healthy(ctx context.Context, url string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	response.Body.Close()
	return response.StatusCode < 500
}
