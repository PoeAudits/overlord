package thoughts

import (
	"os"
	"path/filepath"
	"testing"

	"errors"
	"github.com/PoeAudits/overlord/internal/lifecycle"
	"io/fs"
)

const thoughtsDirSetting = "thoughts_dir"

func TestRegister_ProjectCreatingCreatesStructure(t *testing.T) {
	t.Parallel()
	bus := lifecycle.NewBus()
	Register(bus)

	thoughtsDir := t.TempDir()
	project := "aurora"
	projectPath := t.TempDir()
	writeFile(t, filepath.Join(projectPath, "README.md"), "readme")
	writeFile(t, filepath.Join(projectPath, "AGENTS.md"), "agents")

	result := bus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectCreating,
		ProjectName: project,
		ProjectPath: projectPath,
		Settings:    map[string]string{thoughtsDirSetting: thoughtsDir},
	})
	if result.Err != nil {
		t.Fatalf("project creating handler failed: %v", result.Err)
	}

	assertThoughtsStructureExists(t, thoughtsDir, project)
	assertDocsSymlinks(t, thoughtsDir, project, projectPath)
}

func TestRegister_ProjectInitializingCreatesStructure(t *testing.T) {
	t.Parallel()
	bus := lifecycle.NewBus()
	Register(bus)

	thoughtsDir := t.TempDir()
	project := "ignite"
	projectPath := t.TempDir()
	writeFile(t, filepath.Join(projectPath, "README.md"), "r")
	writeFile(t, filepath.Join(projectPath, "AGENTS.md"), "a")

	result := bus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectInitializing,
		ProjectName: project,
		ProjectPath: projectPath,
		Settings:    map[string]string{thoughtsDirSetting: thoughtsDir},
	})
	if result.Err != nil {
		t.Fatalf("project initializing handler failed: %v", result.Err)
	}

	assertThoughtsStructureExists(t, thoughtsDir, project)
	assertDocsSymlinks(t, thoughtsDir, project, projectPath)
}

func TestRegister_ProjectArchivingMovesThoughts(t *testing.T) {
	t.Parallel()
	bus := lifecycle.NewBus()
	Register(bus)

	thoughtsDir := t.TempDir()
	project := "archive-me"
	projectPath := t.TempDir()
	activePaths := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	ensureDirectories(t, activePaths)
	writeFile(t, filepath.Join(activePaths["plans"], "plan.md"), "plan")

	result := bus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectArchiving,
		ProjectName: project,
		ProjectPath: projectPath,
		Settings:    map[string]string{thoughtsDirSetting: thoughtsDir},
	})
	if result.Err != nil {
		t.Fatalf("project archiving handler failed: %v", result.Err)
	}

	for name, path := range activePaths {
		if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("expected active %s directory to be removed", name)
		}
	}

	archivePaths := expectedPathMap(filepath.Join(thoughtsDir, "archive", project))
	for name, path := range archivePaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected archive %s directory: %v", name, err)
		}
	}
	assertFileContents(t, filepath.Join(archivePaths["plans"], "plan.md"), "plan")
}

func TestRegister_ProjectUnarchivingRestoresThoughts(t *testing.T) {
	t.Parallel()
	bus := lifecycle.NewBus()
	Register(bus)

	thoughtsDir := t.TempDir()
	project := "restore-me"
	projectPath := t.TempDir()
	archivePaths := expectedPathMap(filepath.Join(thoughtsDir, "archive", project))
	ensureDirectories(t, archivePaths)
	writeFile(t, filepath.Join(archivePaths["logs"], "entry.md"), "entry")

	result := bus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectUnarchiving,
		ProjectName: project,
		ProjectPath: projectPath,
		Settings:    map[string]string{thoughtsDirSetting: thoughtsDir},
	})
	if result.Err != nil {
		t.Fatalf("project unarchiving handler failed: %v", result.Err)
	}

	activePaths := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	for name, path := range activePaths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected active %s directory after restore: %v", name, err)
		}
	}
	assertFileContents(t, filepath.Join(activePaths["logs"], "entry.md"), "entry")
	if _, err := os.Stat(filepath.Join(thoughtsDir, "archive", project)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected archive base to be removed after restore")
	}
}

func TestRegister_PostEventErrorsBecomeWarnings(t *testing.T) {
	t.Parallel()
	bus := lifecycle.NewBus()
	Register(bus)

	thoughtsDir := t.TempDir()
	project := "warn"
	docsPath := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))["docs"]
	if err := os.MkdirAll(docsPath, 0o755); err != nil {
		t.Fatalf("failed to create docs path: %v", err)
	}
	writeFile(t, filepath.Join(docsPath, "README.md"), "conflict")

	result := bus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectArchived,
		ProjectName: project,
		ProjectPath: t.TempDir(),
		Settings:    map[string]string{thoughtsDirSetting: thoughtsDir},
	})
	if result.Err != nil {
		t.Fatalf("post-event error should be warning, got err: %v", result.Err)
	}
	if len(result.Warnings) == 0 {
		t.Fatalf("expected warning for post-event handler failure")
	}
}

func assertThoughtsStructureExists(t *testing.T, thoughtsDir, project string) {
	t.Helper()
	paths := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))
	for name, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
	}
}

func assertDocsSymlinks(t *testing.T, thoughtsDir, project, projectPath string) {
	t.Helper()
	docsDir := expectedPathMap(filepath.Join(thoughtsDir, "projects", project))["docs"]
	assertSymlinkTarget(t, filepath.Join(docsDir, "README.md"), filepath.Join(projectPath, "README.md"))
	assertSymlinkTarget(t, filepath.Join(docsDir, "AGENTS.md"), filepath.Join(projectPath, "AGENTS.md"))
}
