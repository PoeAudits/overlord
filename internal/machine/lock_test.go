package machine

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegistryLockValidate(t *testing.T) {
	tests := []struct {
		name    string
		lock    *RegistryLock
		wantErr bool
	}{
		{
			name: "valid lock",
			lock: &RegistryLock{
				Owner:     "main",
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
		{
			name:    "nil lock",
			lock:    nil,
			wantErr: true,
		},
		{
			name: "missing owner",
			lock: &RegistryLock{
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			},
			wantErr: true,
		},
		{
			name: "invalid timestamp",
			lock: &RegistryLock{
				Owner:     "main",
				UpdatedAt: "not-a-time",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.lock.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveLoadLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.lock.yaml")

	lock := NewRegistryLock("main", "transfer")
	if err := SaveLock(path, lock); err != nil {
		t.Fatalf("SaveLock() error = %v", err)
	}

	loaded, err := LoadLock(path)
	if err != nil {
		t.Fatalf("LoadLock() error = %v", err)
	}

	if loaded.Owner != "main" {
		t.Fatalf("owner = %q, want main", loaded.Owner)
	}
	if loaded.Note != "transfer" {
		t.Fatalf("note = %q, want transfer", loaded.Note)
	}
}

func TestLoadLockMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.lock.yaml")

	lock, err := LoadLock(path)
	if err != nil {
		t.Fatalf("LoadLock() error = %v", err)
	}
	if lock != nil {
		t.Fatalf("expected nil lock for missing file")
	}
}

func TestLoadLockMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registry.lock.yaml")

	if err := os.WriteFile(path, []byte("owner: [bad"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadLock(path); err == nil {
		t.Fatalf("expected parse error")
	}
}
