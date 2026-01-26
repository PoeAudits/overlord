package machine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRoleIsValid(t *testing.T) {
	tests := []struct {
		role  Role
		valid bool
	}{
		{RoleStorage, true},
		{RoleWorkingSet, true},
		{Role("invalid"), false},
		{Role(""), false},
		{Role("STORAGE"), false}, // Case sensitive
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			if got := tt.role.IsValid(); got != tt.valid {
				t.Errorf("Role.IsValid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestMachineConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  MachineConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid storage config",
			config: MachineConfig{
				Name: "storage-server",
				Role: RoleStorage,
			},
			wantErr: false,
		},
		{
			name: "valid working-set config",
			config: MachineConfig{
				Name:        "laptop",
				Role:        RoleWorkingSet,
				StorageHost: "storage-server.local",
			},
			wantErr: false,
		},
		{
			name: "empty name",
			config: MachineConfig{
				Name: "",
				Role: RoleStorage,
			},
			wantErr: true,
			errMsg:  "machine name cannot be empty",
		},
		{
			name: "invalid role",
			config: MachineConfig{
				Name: "test",
				Role: Role("invalid"),
			},
			wantErr: true,
			errMsg:  "invalid role",
		},
		{
			name: "working-set without storage_host",
			config: MachineConfig{
				Name: "laptop",
				Role: RoleWorkingSet,
			},
			wantErr: true,
			errMsg:  "storage_host is required for working-set role",
		},
		{
			name: "storage with storage_host",
			config: MachineConfig{
				Name:        "storage-server",
				Role:        RoleStorage,
				StorageHost: "should-not-be-set",
			},
			wantErr: true,
			errMsg:  "storage_host should not be set for storage role",
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
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("MachineConfig.Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

func TestSave(t *testing.T) {
	t.Run("valid storage config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		config := &MachineConfig{
			Name: "test-storage",
			Role: RoleStorage,
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
		if !contains(content, "name: test-storage") {
			t.Errorf("saved file does not contain expected name, got:\n%s", content)
		}
		if !contains(content, "role: storage") {
			t.Errorf("saved file does not contain expected role, got:\n%s", content)
		}
	})

	t.Run("valid working-set config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		config := &MachineConfig{
			Name:        "test-laptop",
			Role:        RoleWorkingSet,
			StorageHost: "storage.example.com",
		}

		if err := Save(configPath, config); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}

		content := string(data)
		if !contains(content, "role: working-set") {
			t.Errorf("saved file does not contain expected role, got:\n%s", content)
		}
		if !contains(content, "storage_host: storage.example.com") {
			t.Errorf("saved file does not contain expected storage_host, got:\n%s", content)
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
			Role: RoleWorkingSet,
			// Missing StorageHost
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
			Role: RoleStorage,
		}

		if err := Save(configPath, config); err != nil {
			t.Fatalf("Save() error = %v, want nil", err)
		}

		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			t.Fatal("config file was not created in nested directory")
		}
	})

	t.Run("round trip save then load", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		original := &MachineConfig{
			Name:        "round-trip-test",
			Role:        RoleWorkingSet,
			StorageHost: "my-server.local",
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
		if loaded.Role != original.Role {
			t.Errorf("Role = %v, want %v", loaded.Role, original.Role)
		}
		if loaded.StorageHost != original.StorageHost {
			t.Errorf("StorageHost = %v, want %v", loaded.StorageHost, original.StorageHost)
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

		if config.Role != RoleStorage {
			t.Errorf("Load() default role = %v, want %v", config.Role, RoleStorage)
		}

		if config.StorageHost != "" {
			t.Errorf("Load() default storage_host = %v, want empty", config.StorageHost)
		}
	})

	t.Run("valid storage config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		content := `name: test-storage
role: storage
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		config, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if config.Name != "test-storage" {
			t.Errorf("Load() name = %v, want %v", config.Name, "test-storage")
		}

		if config.Role != RoleStorage {
			t.Errorf("Load() role = %v, want %v", config.Role, RoleStorage)
		}
	})

	t.Run("valid working-set config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		content := `name: test-laptop
role: working-set
storage_host: storage.example.com
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		config, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v, want nil", err)
		}

		if config.Name != "test-laptop" {
			t.Errorf("Load() name = %v, want %v", config.Name, "test-laptop")
		}

		if config.Role != RoleWorkingSet {
			t.Errorf("Load() role = %v, want %v", config.Role, RoleWorkingSet)
		}

		if config.StorageHost != "storage.example.com" {
			t.Errorf("Load() storage_host = %v, want %v", config.StorageHost, "storage.example.com")
		}
	})

	t.Run("invalid config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "machine.yaml")

		// Missing required storage_host for working-set
		content := `name: test-laptop
role: working-set
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
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
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
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

func TestExpandPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	// Compute expected absolute path for relative path test
	relAbsPath, err := filepath.Abs("config/file.yaml")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		path     string
		expected string
		wantErr  bool
	}{
		{
			name:     "tilde only",
			path:     "~",
			expected: homeDir,
			wantErr:  false,
		},
		{
			name:     "tilde with path",
			path:     "~/.config/overlord",
			expected: filepath.Join(homeDir, ".config/overlord"),
			wantErr:  false,
		},
		{
			name:     "absolute path",
			path:     "/etc/config",
			expected: "/etc/config",
			wantErr:  false,
		},
		{
			name:     "relative path",
			path:     "config/file.yaml",
			expected: relAbsPath,
			wantErr:  false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "tilde user expansion",
			path:    "~user/path",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("expandPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config, err := defaultConfig()
	if err != nil {
		t.Fatalf("defaultConfig() error = %v, want nil", err)
	}

	if config.Name == "" {
		t.Error("defaultConfig() returned config with empty name")
	}

	if config.Role != RoleStorage {
		t.Errorf("defaultConfig() role = %v, want %v", config.Role, RoleStorage)
	}

	if config.StorageHost != "" {
		t.Errorf("defaultConfig() storage_host = %v, want empty", config.StorageHost)
	}

	// Verify it's valid
	if err := config.Validate(); err != nil {
		t.Errorf("defaultConfig() returned invalid config: %v", err)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
