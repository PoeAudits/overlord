package stack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenCodeBinary_PrefersStableBinary(t *testing.T) {
	tmp := t.TempDir()
	npxDir := filepath.Join(tmp, "npx", ".npm", "_npx", "abc", "node_modules", ".bin")
	stableDir := filepath.Join(tmp, "stable")
	if err := os.MkdirAll(npxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stableDir, 0o755); err != nil {
		t.Fatal(err)
	}

	npxBin := filepath.Join(npxDir, "opencode")
	stableBin := filepath.Join(stableDir, "opencode")
	if err := os.WriteFile(npxBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stableBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", prevPath) })
	if err := os.Setenv("PATH", npxDir+string(os.PathListSeparator)+stableDir); err != nil {
		t.Fatal(err)
	}

	got := resolveOpenCodeBinary()
	if got != stableBin {
		t.Fatalf("resolveOpenCodeBinary() = %q, want %q", got, stableBin)
	}
}

func TestResolveOpenCodeBinary_SkipsNPXWrapperScript(t *testing.T) {
	tmp := t.TempDir()
	wrapperDir := filepath.Join(tmp, "wrapper")
	stableDir := filepath.Join(tmp, "stable")
	if err := os.MkdirAll(wrapperDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stableDir, 0o755); err != nil {
		t.Fatal(err)
	}

	wrapper := filepath.Join(wrapperDir, "opencode")
	stable := filepath.Join(stableDir, "opencode")
	if err := os.WriteFile(wrapper, []byte("#!/bin/bash\nexec npx --yes opencode-ai \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stable, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	prevPath := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", prevPath) })
	if err := os.Setenv("PATH", wrapperDir+string(os.PathListSeparator)+stableDir); err != nil {
		t.Fatal(err)
	}

	got := resolveOpenCodeBinary()
	if got != stable {
		t.Fatalf("resolveOpenCodeBinary() = %q, want %q", got, stable)
	}
}
