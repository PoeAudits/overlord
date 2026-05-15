package stack

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestStopStack_NoSession(t *testing.T) {
	prevLookPath := commandLookPath
	prevExec := commandExec
	t.Cleanup(func() {
		commandLookPath = prevLookPath
		commandExec = prevExec
	})

	commandLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	commandExec = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 0")
	}

	result, err := StopStack(nil, tmux.WithRunner(sessionNotExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SessionStopped {
		t.Fatal("expected SessionStopped=false")
	}
	if !result.TailscaleReset {
		t.Fatal("expected TailscaleReset=true")
	}
}

func TestStopStack_KillsSession(t *testing.T) {
	prevLookPath := commandLookPath
	prevExec := commandExec
	t.Cleanup(func() {
		commandLookPath = prevLookPath
		commandExec = prevExec
	})
	commandLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	commandExec = func(name string, args ...string) *exec.Cmd {
		return exec.Command("sh", "-c", "exit 0")
	}

	var commands []string
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil, nil
	})

	result, err := StopStack(nil, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.SessionStopped {
		t.Fatal("expected SessionStopped=true")
	}
	if !result.TailscaleReset {
		t.Fatal("expected TailscaleReset=true")
	}

	foundKill := false
	for _, c := range commands {
		if strings.Contains(c, "kill-session") {
			foundKill = true
			break
		}
	}
	if !foundKill {
		t.Fatal("expected kill-session command")
	}
}

func TestStopStack_WarnsWhenTailscaleUnavailable(t *testing.T) {
	prevLookPath := commandLookPath
	prevExec := commandExec
	t.Cleanup(func() {
		commandLookPath = prevLookPath
		commandExec = prevExec
	})

	commandLookPath = func(string) (string, error) { return "", errors.New("missing") }

	result, err := StopStack(nil, tmux.WithRunner(sessionNotExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning when tailscale is unavailable")
	}
}

func TestStopStack_ReturnsErrorWhenKillFails(t *testing.T) {
	call := 0
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		call++
		switch call {
		case 1, 2:
			return nil, nil
		case 3:
			return []byte("kill failed"), errors.New("exit status 1")
		default:
			return nil, nil
		}
	})

	_, err := StopStack(nil, tmux.WithRunner(runner))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "killing session StartServer: kill failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopStack_WarnsWhenTailscaleResetFails(t *testing.T) {
	prevLookPath := commandLookPath
	prevExec := commandExec
	t.Cleanup(func() {
		commandLookPath = prevLookPath
		commandExec = prevExec
	})

	commandLookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	commandExec = func(name string, args ...string) *exec.Cmd {
		return commandWithOutputAndExit("permission denied", 1)
	}

	result, err := StopStack(nil, tmux.WithRunner(sessionNotExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TailscaleReset {
		t.Fatal("expected TailscaleReset=false")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(result.Warnings))
	}
	if !strings.Contains(result.Warnings[0], "tailscale serve reset failed") || !strings.Contains(result.Warnings[0], "permission denied") {
		t.Fatalf("unexpected warning: %v", result.Warnings[0])
	}
}
