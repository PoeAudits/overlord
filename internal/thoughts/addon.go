package thoughts

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/lifecycle"
)

const thoughtsDirSettingKey = "thoughts_dir"

// Register attaches the thoughts handlers to the provided lifecycle bus.
func Register(bus *lifecycle.Bus) {
	if bus == nil {
		return
	}

	bus.Subscribe(lifecycle.ProjectCreating, handleCreateOrInit)
	bus.Subscribe(lifecycle.ProjectInitializing, handleCreateOrInit)
	bus.Subscribe(lifecycle.ProjectArchiving, handleArchiving)
	bus.Subscribe(lifecycle.ProjectUnarchiving, handleUnarchiving)
	bus.Subscribe(lifecycle.ProjectArchived, handlePostPathUpdate)
	bus.Subscribe(lifecycle.ProjectUnarchived, handlePostPathUpdate)
}

func init() {
	Register(lifecycle.DefaultBus)
}

func handleCreateOrInit(event lifecycle.Event) error {
	thoughtsDir := event.Settings[thoughtsDirSettingKey]
	if thoughtsDir == "" {
		return nil
	}

	if err := CreateStructure(thoughtsDir, event.ProjectName); err != nil {
		return fmt.Errorf("failed to create thoughts structure: %w", err)
	}

	if err := EnsureProjectSymlink(thoughtsDir, event.ProjectName, event.ProjectPath); err != nil {
		return fmt.Errorf("failed to ensure thoughts symlinks: %w", err)
	}

	return nil
}

func handleArchiving(event lifecycle.Event) error {
	thoughtsDir := event.Settings[thoughtsDirSettingKey]
	if thoughtsDir == "" {
		return nil
	}

	if err := MoveToArchive(thoughtsDir, event.ProjectName); err != nil {
		return fmt.Errorf("failed to archive thoughts: %w", err)
	}

	return nil
}

func handleUnarchiving(event lifecycle.Event) error {
	thoughtsDir := event.Settings[thoughtsDirSettingKey]
	if thoughtsDir == "" {
		return nil
	}

	if err := MoveFromArchive(thoughtsDir, event.ProjectName); err != nil {
		return fmt.Errorf("failed to restore thoughts from archive: %w", err)
	}

	return nil
}

func handlePostPathUpdate(event lifecycle.Event) error {
	thoughtsDir := event.Settings[thoughtsDirSettingKey]
	if thoughtsDir == "" {
		return nil
	}

	archived := event.Type == lifecycle.ProjectArchived
	if err := UpdateSymlinksForState(thoughtsDir, event.ProjectName, event.ProjectPath, archived); err != nil {
		return fmt.Errorf("failed to update thoughts symlinks: %w", err)
	}

	return nil
}
