package machine

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMachineConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  MachineConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid minimal config",
			config: MachineConfig{
				Name: "desktop",
			},
			wantErr: false,
		},
		{
			name: "valid syncthing config",
			config: MachineConfig{
				Name: "vps",
				Role: RoleVPS,
				Syncthing: SyncthingConfig{
					DeviceID: "XXXX-XXXX-XXXX-XXXX",
					APIURL:   "http://localhost:8384",
					APIKey:   "key",
					Address:  "tcp://100.100.100.100:22000",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid role",
			config: MachineConfig{
				Name: "desktop",
				Role: Role("bad-role"),
			},
			wantErr: true,
			errMsg:  "invalid machine role",
		},
		{
			name: "empty name",
			config: MachineConfig{
				Name: "",
			},
			wantErr: true,
			errMsg:  "machine name cannot be empty",
		},
		{
			name: "invalid syncthing config (missing api_key)",
			config: MachineConfig{
				Name: "test",
				Syncthing: SyncthingConfig{
					DeviceID: "XXXX-XXXX-XXXX-XXXX",
					APIURL:   "http://localhost:8384",
					Address:  "tcp://100.100.100.100:22000",
				},
			},
			wantErr: true,
			errMsg:  "syncthing config requires device_id, api_url, and api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MachineConfig.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("MachineConfig.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSave(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		config := &MachineConfig{
			Name: "test-machine",
		}

		if err := Save(configPath, config); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		// Verify file exists and contents
		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "name: test-machine") {
			t.Errorf("saved file does not contain expected name, got:\n%s", content)
		}
	})

	t.Run("valid syncthing config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		config := &MachineConfig{
			Name: "test-laptop",
			Syncthing: SyncthingConfig{
				DeviceID: "XXXX-XXXX-XXXX-XXXX",
				APIURL:   "http://localhost:8384",
				APIKey:   "key",
				Address:  "tcp://100.100.100.100:22000",
			},
		}

		if err := Save(configPath, config); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "syncthing:") {
			t.Errorf("saved file does not contain expected syncthing config, got:\n%s", content)
		}
	})

	t.Run("nil config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		err := Save(configPath, nil)
		if err == nil {
			t.Fatal("Save() error = nil, want error for nil config")
		}
	})

	t.Run("invalid config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		config := &MachineConfig{
			Name: "laptop",
			Syncthing: SyncthingConfig{
				DeviceID: "XXXX-XXXX-XXXX-XXXX",
				APIURL:   "http://localhost:8384",
				Address:  "tcp://100.100.100.100:22000",
			},
		}

		err := Save(configPath, config)
		if err == nil {
			t.Fatal("Save() error = nil, want validation error")
		}
	})

	t.Run("creates parent directories", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "nested", "dirs", "machine.yaml")

		config := &MachineConfig{
			Name: "test",
		}

		if err := Save(configPath, config); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		if _, err := os.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
			t.Fatal("config file was not created in nested directory")
		}
	})

	t.Run("round trip save then load", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		original := &MachineConfig{
			Name: "round-trip-test",
		}

		if err := Save(configPath, original); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if loaded.Name != original.Name {
			t.Errorf("Name = %v, want %v", loaded.Name, original.Name)
		}
	})
}

func TestLoad(t *testing.T) {
	t.Run("file not found returns default", func(t *testing.T) {
		// Use a path that definitely doesn't exist
		nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist.yaml")

		config, err := Load(nonExistentPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if config.Name == "" {
			t.Error("Load() returned config with empty name")
		}

	})

	t.Run("valid storage config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		content := `name: test-storage
`
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		config, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if config.Name != "test-storage" {
			t.Errorf("Load() name = %v, want %v", config.Name, "test-storage")
		}

	})

	t.Run("invalid config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		content := `name: test-laptop
syncthing:
  device_id: "XXXX-XXXX-XXXX-XXXX"
  api_url: "http://localhost:8384"
  address: "tcp://100.100.100.100:22000"
`
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(configPath)
		if err == nil {
			t.Error("Load() error = nil, want error for invalid config")
		}
	})

	t.Run("malformed YAML", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		content := `name: test
role: [invalid yaml structure
`
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		_, err := Load(configPath)
		if err == nil {
			t.Error("Load() error = nil, want error for malformed YAML")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		_, err := Load("")
		if err == nil {
			t.Error("Load() error = nil, want error for empty path")
		}
	})
}
