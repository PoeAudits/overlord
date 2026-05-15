package fileutil

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFormatPathWithTilde(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	joinedHome := filepath.Join(homeDir, "projects", "demo")
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "under home directory",
			input:    joinedHome,
			expected: "~" + strings.TrimPrefix(joinedHome, homeDir),
		},
		{
			name:     "outside home left unchanged",
			input:    filepath.Join(os.TempDir(), "out-of-home"),
			expected: filepath.Join(os.TempDir(), "out-of-home"),
		},
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := FormatPathWithTilde(tc.input)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestResolveProjectPath(t *testing.T) {
	t.Parallel()

	base := filepath.Join(os.TempDir(), "base")
	absoluteProject := filepath.Join(base, "abs-project")
	d := []struct {
		name     string
		project  string
		baseDir  string
		expected string
	}{
		{
			name:     "relative project path",
			project:  "alpha",
			baseDir:  base,
			expected: filepath.Join(base, "alpha"),
		},
		{
			name:     "absolute project path",
			project:  absoluteProject,
			baseDir:  base,
			expected: absoluteProject,
		},
	}

	for _, tc := range d {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveProjectPath(tc.project, tc.baseDir); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestExpandProjectPath(t *testing.T) {
	t.Parallel()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	baseDir := filepath.Join("~", "Overlord")
	relativeProject := filepath.Join("projects", "gamma")
	absoluteProject := filepath.Join(homeDir, "custom", "delta")

	cases := []struct {
		name        string
		projectPath string
		baseDir     string
		expected    string
		wantErr     bool
	}{
		{
			name:        "expands tilde base dir",
			projectPath: relativeProject,
			baseDir:     baseDir,
			expected:    filepath.Clean(filepath.Join(homeDir, "Overlord", relativeProject)),
		},
		{
			name:        "returns absolute project path as-is",
			projectPath: absoluteProject,
			baseDir:     baseDir,
			expected:    absoluteProject,
		},
		{
			name:        "errors when base dir invalid",
			projectPath: relativeProject,
			baseDir:     "",
			wantErr:     true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExpandProjectPath(tc.projectPath, tc.baseDir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestCopyFileWithMode(t *testing.T) {
	t.Run("copies file contents and mode", func(t *testing.T) {
		srcDir := t.TempDir()
		dstDir := t.TempDir()

		srcPath := filepath.Join(srcDir, "source.txt")
		content := []byte("hello world")
		mode := os.FileMode(0o640)
		writeFileWithMode(t, srcPath, content, mode)

		dstPath := filepath.Join(dstDir, "copied.txt")
		if err := CopyFileWithMode(srcPath, dstPath); err != nil {
			t.Fatalf("CopyFileWithMode failed: %v", err)
		}

		got, err := os.ReadFile(dstPath)
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}

		if string(got) != string(content) {
			t.Fatalf("expected %q, got %q", content, got)
		}

		info, err := os.Stat(dstPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}

		if info.Mode().Perm() != mode {
			t.Fatalf("expected mode %v, got %v", mode, info.Mode().Perm())
		}
	})

	t.Run("removes partial destination on copy failure", func(t *testing.T) {
		origCopy := copyFunc
		copyFunc = func(dst io.Writer, src io.Reader) (int64, error) {
			if _, err := dst.Write([]byte("partial")); err != nil {
				return 0, err
			}
			return 7, errors.New("simulated failure")
		}
		t.Cleanup(func() {
			copyFunc = origCopy
		})

		srcDir := t.TempDir()
		dstDir := t.TempDir()
		srcPath := filepath.Join(srcDir, "source.txt")
		writeFileWithMode(t, srcPath, []byte("content"), 0o600)

		dstPath := filepath.Join(dstDir, "copied.txt")
		err := CopyFileWithMode(srcPath, dstPath)
		if err == nil {
			t.Fatal("expected error from CopyFileWithMode, got nil")
		}

		if _, statErr := os.Stat(dstPath); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("expected destination file to be removed, stat err = %v", statErr)
		}
	})
}

func TestCopyDirectory(t *testing.T) {
	t.Run("copies nested structure", func(t *testing.T) {
		srcDir := t.TempDir()
		nestedDir := filepath.Join(srcDir, "nested")
		if err := os.MkdirAll(nestedDir, 0o750); err != nil {
			t.Fatalf("failed to create nested dir: %v", err)
		}

		srcFile1 := filepath.Join(srcDir, "root.txt")
		srcFile2 := filepath.Join(nestedDir, "child.txt")
		writeFileWithMode(t, srcFile1, []byte("root"), 0o640)
		writeFileWithMode(t, srcFile2, []byte("child"), 0o600)

		dstDir := filepath.Join(t.TempDir(), "dest")
		if err := CopyDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("CopyDirectory failed: %v", err)
		}

		for _, rel := range []string{"root.txt", filepath.Join("nested", "child.txt")} {
			data, err := os.ReadFile(filepath.Join(dstDir, rel))
			if err != nil {
				t.Fatalf("failed to read copied file %s: %v", rel, err)
			}
			if len(data) == 0 {
				t.Fatalf("copied file %s is empty", rel)
			}
		}

		info, err := os.Stat(filepath.Join(dstDir, "nested"))
		if err != nil {
			t.Fatalf("stat nested dir failed: %v", err)
		}
		if info.Mode().Perm() != 0o750 {
			t.Fatalf("expected nested dir mode 0750, got %v", info.Mode().Perm())
		}
	})

	t.Run("errors when source missing", func(t *testing.T) {
		if err := CopyDirectory("/not-real", filepath.Join(t.TempDir(), "dst")); err == nil {
			t.Fatal("expected error for missing source directory")
		}
	})
}

func TestMoveDirectory(t *testing.T) {
	t.Run("moves directory and removes source", func(t *testing.T) {
		srcDir := createProjectDir(t)
		dstDir := filepath.Join(t.TempDir(), "moved")

		if err := MoveDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("MoveDirectory failed: %v", err)
		}

		assertFileExists(t, filepath.Join(dstDir, "data", "file.txt"))
		if _, err := os.Stat(srcDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected source to be removed, stat err = %v", err)
		}
	})

	t.Run("creates destination parent directories", func(t *testing.T) {
		srcDir := createProjectDir(t)
		parent := filepath.Join(t.TempDir(), "parent", "nested")
		dstDir := filepath.Join(parent, "final")

		if err := MoveDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("MoveDirectory failed: %v", err)
		}

		assertFileExists(t, filepath.Join(dstDir, "data", "file.txt"))
	})

	t.Run("returns error when source missing", func(t *testing.T) {
		if err := MoveDirectory("/does-not-exist", filepath.Join(t.TempDir(), "dst")); err == nil {
			t.Fatal("expected error for missing source directory")
		}
	})

	t.Run("falls back to copy and remove on cross-device errors", func(t *testing.T) {
		origRename := renameFunc
		renameFunc = func(_, _ string) error {
			return &os.LinkError{Err: syscall.EXDEV}
		}
		t.Cleanup(func() {
			renameFunc = origRename
		})

		srcDir := createProjectDir(t)
		dstDir := filepath.Join(t.TempDir(), "cross-device")

		if err := MoveDirectory(srcDir, dstDir); err != nil {
			t.Fatalf("MoveDirectory failed: %v", err)
		}

		assertFileExists(t, filepath.Join(dstDir, "data", "file.txt"))
		if _, err := os.Stat(srcDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected source to be removed after fallback, stat err = %v", err)
		}
	})
}

func TestMoveDirectory_CrossDeviceCopyFailure(t *testing.T) {
	// Override renameFunc to simulate a cross-device error so MoveDirectory
	// falls back to the copy-then-remove path.
	origRename := renameFunc
	renameFunc = func(_, _ string) error {
		return &os.LinkError{Err: syscall.EXDEV}
	}
	t.Cleanup(func() { renameFunc = origRename })

	// Override copyFunc so the copy step fails partway through.
	origCopy := copyFunc
	copyFunc = func(_ io.Writer, _ io.Reader) (int64, error) {
		return 0, errors.New("simulated copy failure")
	}
	t.Cleanup(func() { copyFunc = origCopy })

	srcDir := createProjectDir(t)
	dstDir := filepath.Join(t.TempDir(), "should-not-exist")

	err := MoveDirectory(srcDir, dstDir)
	if err == nil {
		t.Fatal("expected MoveDirectory to return an error when copy fails")
	}

	// Source must still exist because the copy failed before removal.
	if _, statErr := os.Stat(srcDir); statErr != nil {
		t.Fatalf("source directory should still exist after failed copy, got: %v", statErr)
	}
}

func TestAtomicWriteFile(t *testing.T) {
	t.Run("replaces existing file contents", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		writeFileWithMode(t, path, []byte("old"), 0o644)

		if err := AtomicWriteFile(path, []byte("new"), 0o600); err != nil {
			t.Fatalf("AtomicWriteFile() returned error: %v", err)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("failed to read destination file: %v", err)
		}
		if string(data) != "new" {
			t.Fatalf("file contents = %q, want %q", data, "new")
		}
		if _, err := os.Stat(path + ".tmp"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected temp file to be absent, stat err = %v", err)
		}
	})

	t.Run("removes temp file when rename fails", func(t *testing.T) {
		origRename := renameFunc
		renameFunc = func(_, _ string) error {
			return errors.New("rename failed")
		}
		t.Cleanup(func() {
			renameFunc = origRename
		})

		path := filepath.Join(t.TempDir(), "config.yaml")
		writeFileWithMode(t, path, []byte("old"), 0o644)

		err := AtomicWriteFile(path, []byte("new"), 0o600)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to rename temp file") {
			t.Fatalf("unexpected error: %v", err)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("failed to read original file: %v", readErr)
		}
		if string(data) != "old" {
			t.Fatalf("original file contents = %q, want %q", data, "old")
		}
		if _, statErr := os.Stat(path + ".tmp"); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("expected temp file cleanup, stat err = %v", statErr)
		}
	})
}

func createProjectDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	srcDir := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(srcDir, "data"), 0o755); err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	writeFileWithMode(t, filepath.Join(srcDir, "data", "file.txt"), []byte("payload"), 0o640)
	return srcDir
}

func writeFileWithMode(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent directories: %v", err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
}
