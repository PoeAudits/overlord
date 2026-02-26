package machine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultRegistryLockPath is the default location for the registry lock file.
	DefaultRegistryLockPath = "~/.config/overlord/registry.lock.yaml"
)

// RegistryLock is the single-writer lock metadata for registry mutations.
type RegistryLock struct {
	Owner     string `yaml:"owner"`
	UpdatedAt string `yaml:"updated_at"`
	Note      string `yaml:"note,omitempty"`
}

// Validate checks if lock contents are valid.
func (l *RegistryLock) Validate() error {
	if l == nil {
		return fmt.Errorf("registry lock cannot be nil")
	}
	if l.Owner == "" {
		return fmt.Errorf("owner is required")
	}
	if l.UpdatedAt == "" {
		return fmt.Errorf("updated_at is required")
	}
	if _, err := time.Parse(time.RFC3339, l.UpdatedAt); err != nil {
		return fmt.Errorf("updated_at must be RFC3339: %w", err)
	}
	return nil
}

// LoadLock reads lock file from path.
// Missing file returns (nil, nil).
func LoadLock(path string) (*RegistryLock, error) {
	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	if _, err := os.Stat(expandedPath); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	data, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read registry lock file: %w", err)
	}

	var lock RegistryLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse registry lock YAML: %w", err)
	}

	if err := lock.Validate(); err != nil {
		return nil, fmt.Errorf("registry lock validation failed: %w", err)
	}

	return &lock, nil
}

// SaveLock writes lock file atomically.
func SaveLock(path string, lock *RegistryLock) error {
	if lock == nil {
		return fmt.Errorf("registry lock cannot be nil")
	}
	if err := lock.Validate(); err != nil {
		return fmt.Errorf("cannot save invalid registry lock: %w", err)
	}

	expandedPath, err := registry.ExpandPath(path)
	if err != nil {
		return fmt.Errorf("failed to expand path: %w", err)
	}

	dir := filepath.Dir(expandedPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(lock)
	if err != nil {
		return fmt.Errorf("failed to marshal registry lock to YAML: %w", err)
	}

	// Backup existing file to .bak before overwriting
	backupPath := expandedPath + ".bak"
	if existing, err := os.ReadFile(expandedPath); err == nil {
		if err := os.WriteFile(backupPath, existing, 0o644); err != nil {
			return fmt.Errorf("failed to create backup file: %w", err)
		}
	}

	if err := fileutil.AtomicWriteFile(expandedPath, data, 0o644); err != nil {
		return err
	}

	return nil
}

// NewRegistryLock creates a lock owned by the given machine name.
func NewRegistryLock(owner, note string) *RegistryLock {
	return &RegistryLock{
		Owner:     owner,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Note:      note,
	}
}
