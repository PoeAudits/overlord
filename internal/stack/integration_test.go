package stack

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// --- Test 1: Default services on fresh config ---

func TestIntegration_DefaultServicesOnFreshConfig(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistent := filepath.Join(tmpDir, "no-such-file.yaml")

	cfg, err := Load(nonExistent)
	if err != nil {
		t.Fatalf("Load returned error for non-existent file: %v", err)
	}

	services := DefaultServices(cfg)
	if len(services) != 4 {
		t.Fatalf("expected 4 default services, got %d", len(services))
	}

	expectedOrder := []struct {
		name       string
		windowName string
	}{
		{"opencode", "OpenCode"},
		{"frontend", "FrontEnd"},
		{"tailscale", "Tailscale"},
		{"ollama", "Ollama"},
	}

	for i, want := range expectedOrder {
		got := services[i]
		if got.Name() != want.name {
			t.Errorf("service[%d].Name() = %q, want %q", i, got.Name(), want.name)
		}
		if got.WindowName() != want.windowName {
			t.Errorf("service[%d].WindowName() = %q, want %q", i, got.WindowName(), want.windowName)
		}
	}
}

// --- Test 2: Add and persist projects ---

func TestIntegration_AddAndPersistProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stack.yaml")

	cfg := DefaultConfig()
	cfg.Projects = append(cfg.Projects, "alpha", "beta", "gamma")

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	want := []string{"alpha", "beta", "gamma"}
	if len(loaded.Projects) != len(want) {
		t.Fatalf("expected %d projects, got %d", len(want), len(loaded.Projects))
	}
	for i, p := range want {
		if loaded.Projects[i] != p {
			t.Errorf("Projects[%d] = %q, want %q", i, loaded.Projects[i], p)
		}
	}
}

// --- Test 3: Remove and persist projects ---

func TestIntegration_RemoveAndPersistProjects(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stack.yaml")

	cfg := DefaultConfig()
	cfg.Projects = []string{"alpha", "beta", "gamma"}

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("initial Save failed: %v", err)
	}

	// Remove "beta" from the projects list.
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var filtered []string
	for _, p := range loaded.Projects {
		if p != "beta" {
			filtered = append(filtered, p)
		}
	}
	loaded.Projects = filtered

	if err := Save(configPath, loaded); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	reloaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}

	want := []string{"alpha", "gamma"}
	if len(reloaded.Projects) != len(want) {
		t.Fatalf("expected %d projects after removal, got %d", len(want), len(reloaded.Projects))
	}
	for i, p := range want {
		if reloaded.Projects[i] != p {
			t.Errorf("Projects[%d] = %q, want %q", i, reloaded.Projects[i], p)
		}
	}
}

// --- Test 4: StartStack with no projects configured ---

func TestIntegration_StartStackNoProjects(t *testing.T) {
	// Reset the lifecycle bus to avoid interference from other tests.
	defer func() { *lifecycle.DefaultBus = *lifecycle.NewBus() }()

	var commands []string
	sessionCreated := false
	runner := mockRunner(func(name string, args ...string) ([]byte, error) {
		cmd := name + " " + strings.Join(args, " ")
		commands = append(commands, cmd)
		if name == "tmux" && len(args) > 0 {
			switch args[0] {
			case "has-session":
				if !sessionCreated {
					return nil, &exitError{code: 1}
				}
				return nil, nil
			case "new-session":
				sessionCreated = true
				return nil, nil
			}
		}
		return nil, nil
	})

	cfg := enabledConfig("opencode", "frontend")
	// No projects set — cfg.Projects is nil/empty by construction.

	result, err := StartStack(cfg, tmux.WithRunner(runner))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the tmux session was created.
	foundNewSession := false
	for _, c := range commands {
		if strings.Contains(c, "new-session") {
			foundNewSession = true
		}
	}
	if !foundNewSession {
		t.Error("expected tmux new-session command to be issued")
	}

	// Verify results: each service should be started (not skipped).
	for _, sr := range result.ServiceResults {
		if sr.Status != StatusStarted {
			t.Errorf("service %s: expected 'started', got %q", sr.Name, sr.Status)
		}
	}

	if len(result.ServiceResults) != 2 {
		t.Errorf("expected 2 service results, got %d", len(result.ServiceResults))
	}
}

// --- Test 5: Session already exists (idempotency) ---

func TestIntegration_SessionAlreadyExistsIdempotency(t *testing.T) {
	// Reset the lifecycle bus to avoid interference.
	defer func() { *lifecycle.DefaultBus = *lifecycle.NewBus() }()

	cfg := enabledConfig("opencode", "frontend", "tailscale", "ollama")

	result, err := StartStack(cfg, tmux.WithRunner(sessionExistsRunner()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	services := DefaultServices(cfg)
	if len(result.ServiceResults) != len(services) {
		t.Fatalf("expected %d service results, got %d", len(services), len(result.ServiceResults))
	}

	for i, sr := range result.ServiceResults {
		if sr.Status != StatusSkipped {
			t.Errorf("service %s: expected status 'skipped', got %q", sr.Name, sr.Status)
		}
		// Verify ordering matches DefaultServices.
		if sr.Name != services[i].Name() {
			t.Errorf("result[%d].Name = %q, want %q", i, sr.Name, services[i].Name())
		}
		if sr.DisplayName != services[i].WindowName() {
			t.Errorf("result[%d].DisplayName = %q, want %q", i, sr.DisplayName, services[i].WindowName())
		}
	}

	// Verify no errors are set on skipped services.
	for _, sr := range result.ServiceResults {
		if sr.Error != nil {
			t.Errorf("service %s: expected nil error for skipped service, got: %v", sr.Name, sr.Error)
		}
	}
}
