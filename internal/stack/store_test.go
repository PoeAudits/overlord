package stack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultConfigWhenFileNotExist(t *testing.T) {
	// Use a path that does not exist
	tmpDir := t.TempDir()
	nonExistentPath := filepath.Join(tmpDir, "does-not-exist.yaml")

	config, err := Load(nonExistentPath)
	if err != nil {
		t.Fatalf("Load returned error for non-existent file: %v", err)
	}

	if config.Version != 1 {
		t.Errorf("expected Version 1, got %d", config.Version)
	}
	if len(config.Projects) != 0 {
		t.Errorf("expected empty Projects, got %v", config.Projects)
	}
	if len(config.Services) != 4 {
		t.Errorf("expected 4 default services, got %d", len(config.Services))
	}

	expectedServices := []string{"opencode", "frontend", "tailscale", "ollama"}
	for _, name := range expectedServices {
		svc, ok := config.Services[name]
		if !ok {
			t.Errorf("expected default service %q to exist", name)
			continue
		}
		if !svc.Enabled {
			t.Errorf("expected service %q to be enabled", name)
		}
	}
}

func TestSaveThenLoad_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stack.yaml")

	original := &StackConfig{
		Version:  1,
		Projects: []string{"project-a", "project-b"},
		Services: map[string]ServiceConfig{
			"opencode": {
				Enabled:  true,
				Settings: map[string]string{"key": "value"},
			},
			"frontend": {
				Enabled:  false,
				Settings: map[string]string{},
			},
		},
	}

	if err := Save(configPath, original); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version mismatch: got %d, want %d", loaded.Version, original.Version)
	}
	if len(loaded.Projects) != len(original.Projects) {
		t.Fatalf("Projects length mismatch: got %d, want %d", len(loaded.Projects), len(original.Projects))
	}
	for i, p := range loaded.Projects {
		if p != original.Projects[i] {
			t.Errorf("Projects[%d] mismatch: got %q, want %q", i, p, original.Projects[i])
		}
	}
	if len(loaded.Services) != len(original.Services) {
		t.Fatalf("Services length mismatch: got %d, want %d", len(loaded.Services), len(original.Services))
	}
	for name, svc := range original.Services {
		loadedSvc, ok := loaded.Services[name]
		if !ok {
			t.Errorf("missing service %q after round-trip", name)
			continue
		}
		if loadedSvc.Enabled != svc.Enabled {
			t.Errorf("service %q Enabled mismatch: got %v, want %v", name, loadedSvc.Enabled, svc.Enabled)
		}
		for k, v := range svc.Settings {
			if loadedSvc.Settings[k] != v {
				t.Errorf("service %q Settings[%q] mismatch: got %q, want %q", name, k, loadedSvc.Settings[k], v)
			}
		}
	}
}

func TestSave_CreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stack.yaml")

	// Write initial config
	first := &StackConfig{
		Version:  1,
		Projects: []string{"first"},
		Services: map[string]ServiceConfig{
			"opencode": {Enabled: true, Settings: map[string]string{}},
		},
	}
	if err := Save(configPath, first); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	// Read what was written so we can compare to backup later
	firstData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read first config: %v", err)
	}

	// Save again with different config — should create .bak
	second := &StackConfig{
		Version:  1,
		Projects: []string{"second"},
		Services: map[string]ServiceConfig{
			"opencode": {Enabled: false, Settings: map[string]string{}},
		},
	}
	if err := Save(configPath, second); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	// Verify .bak exists and contains the first config
	backupPath := configPath + ".bak"
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("failed to read backup file: %v", err)
	}

	if string(backupData) != string(firstData) {
		t.Errorf("backup content does not match first save.\nGot:\n%s\nWant:\n%s", backupData, firstData)
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "stack.yaml")

	config := &StackConfig{
		Version:  1,
		Projects: []string{},
		Services: map[string]ServiceConfig{
			"opencode": {Enabled: true, Settings: map[string]string{}},
		},
	}

	if err := Save(configPath, config); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the final file exists
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file does not exist after save: %v", err)
	}

	// Verify no .tmp file is left behind (atomic rename should have cleaned it up)
	tmpPath := configPath + ".tmp"
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf("temp file %s should not exist after successful atomic write", tmpPath)
	}
}
