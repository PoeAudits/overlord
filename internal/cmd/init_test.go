package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunInitPreservesExistingFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	original := initOpts
	t.Cleanup(func() { initOpts = original })
	initOpts = initFlags{name: "example", langGo: true}
	dir := t.TempDir()
	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("keep this"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInitCmd(&cobra.Command{}, []string{dir}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(readme)
	if err != nil || string(data) != "keep this" {
		t.Fatalf("existing README changed: %q, %v", data, err)
	}
	for _, name := range []string{".tmux.local", "Makefile", "AGENTS.md", "opencode.jsonc"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing generated %s: %v", name, err)
		}
	}
}

func TestRunInitRejectsMissingDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "missing")
	if err := runInitCmd(&cobra.Command{}, []string{path}); err == nil || !strings.Contains(err.Error(), "directory does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("init created missing directory: %v", err)
	}
}
