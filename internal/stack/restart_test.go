package stack

import (
	"errors"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestRestartServices_RestartsNamedServiceWindow(t *testing.T) {
	var commands []string
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil, nil
	})

	result, err := RestartServices(enabledConfig("speech-outbox"), []string{"AliceSpeech"}, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("RestartServices() returned error: %v", err)
	}

	if len(result.ServiceResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.ServiceResults))
	}
	if result.ServiceResults[0].Name != "speech-outbox" || result.ServiceResults[0].Status != StatusRestarted {
		t.Fatalf("unexpected result: %#v", result.ServiceResults[0])
	}

	for _, want := range []string{
		"tmux has-session -t StartServer",
		"tmux kill-window -t StartServer:AliceSpeech",
		"tmux new-window -t StartServer -n AliceSpeech",
		"alice speech-outbox serve --host 127.0.0.1 --port 8090",
	} {
		if !containsCommand(commands, want) {
			t.Fatalf("missing command containing %q in %#v", want, commands)
		}
	}
}

func TestRestartServices_RecreatesPaneCommand(t *testing.T) {
	var commands []string
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return nil, nil
	})

	_, err := RestartServices(enabledConfig("ollama"), []string{"ollama"}, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("RestartServices() returned error: %v", err)
	}

	for _, want := range []string{
		"tmux kill-window -t StartServer:Ollama",
		"tmux new-window -t StartServer -n Ollama",
		"ollama serve",
		"tmux split-window -v -t StartServer:Ollama",
		"ollama run qwen3.5:2b",
	} {
		if !containsCommand(commands, want) {
			t.Fatalf("missing command containing %q in %#v", want, commands)
		}
	}
}

func TestRestartServices_CreatesWindowWhenServiceWindowMissing(t *testing.T) {
	var commands []string
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		if name == "tmux" && len(args) >= 3 && args[0] == "kill-window" {
			return []byte("can't find window: Phoenix"), &exitError{code: 1}
		}
		return nil, nil
	})

	result, err := RestartServices(enabledConfig("phoenix"), []string{"phoenix"}, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("RestartServices() returned error: %v", err)
	}

	if len(result.ServiceResults) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.ServiceResults))
	}
	if result.ServiceResults[0].Name != "phoenix" || result.ServiceResults[0].Status != StatusRestarted {
		t.Fatalf("unexpected result: %#v", result.ServiceResults[0])
	}

	for _, want := range []string{
		"tmux kill-window -t StartServer:Phoenix",
		"tmux new-window -t StartServer -n Phoenix",
		"PHOENIX_HOST=127.0.0.1 PHOENIX_PORT=6006 PHOENIX_WORKING_DIR=/tmp/phoenix-overlord",
		"uv tool run --from arize-phoenix phoenix serve",
	} {
		if !containsCommand(commands, want) {
			t.Fatalf("missing command containing %q in %#v", want, commands)
		}
	}
}

func TestRestartServices_RequiresRunningSession(t *testing.T) {
	_, err := RestartServices(enabledConfig("speech-outbox"), []string{"speech-outbox"}, tmux.WithRunner(sessionNotExistsRunner()))
	if !errors.Is(err, tmux.ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestRestartServices_RejectsUnknownOrDisabledService(t *testing.T) {
	_, err := RestartServices(enabledConfig("speech-outbox"), []string{"opencode"}, tmux.WithRunner(sessionExistsRunner()))
	if err == nil || !strings.Contains(err.Error(), "unknown or disabled service") {
		t.Fatalf("expected unknown service error, got %v", err)
	}
}

func containsCommand(commands []string, want string) bool {
	for _, command := range commands {
		if strings.Contains(command, want) {
			return true
		}
	}
	return false
}
