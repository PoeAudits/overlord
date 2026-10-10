package stack

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestStopStack_NoSessionLeavesHostGlobalServeConfigAlone(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() { commandExec = prevExec })
	commandExec = func(name string, args ...string) *exec.Cmd {
		t.Fatalf("StopStack must not run host-global commands: %s %v", name, args)
		return nil
	}

	result, err := StopStack(nil, tmux.WithRunner(sessionNotExistsRunner()))
	if err != nil {
		t.Fatalf("StopStack() returned error: %v", err)
	}
	if result.SessionStopped {
		t.Fatal("SessionStopped = true, want false")
	}
}

func TestStopStack_KillsSession(t *testing.T) {
	var commands []string
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil, nil
	})

	result, err := StopStack(nil, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("StopStack() returned error: %v", err)
	}
	if !result.SessionStopped {
		t.Fatal("SessionStopped = false, want true")
	}
	if !containsCommand(commands, "tmux kill-session -t "+SessionName) {
		t.Fatalf("missing kill-session in %#v", commands)
	}
}

func TestStopStack_ReturnsErrorWhenKillFails(t *testing.T) {
	call := 0
	runner := mockRunner(func(_ string, _ ...string) ([]byte, error) {
		call++
		if call <= 2 {
			return nil, nil
		}
		return []byte("kill failed"), errors.New("exit status 1")
	})

	_, err := StopStack(nil, tmux.WithRunner(runner))
	if err == nil {
		t.Fatal("StopStack() error = nil, want kill failure")
	}
	if !strings.Contains(err.Error(), "killing session StartServer: kill failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}
