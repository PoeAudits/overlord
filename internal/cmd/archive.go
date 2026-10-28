package cmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/registry"
)

type archiveFlags struct {
	noConfirm bool
}

var archiveOpts archiveFlags

var (
	archiveLoadRegistry = loadRegistry
	archiveSaveRegistry = saveRegistry
)

// archiveCmd represents the archive command
var archiveCmd = &cobra.Command{
	Use:   "archive <name> [name...]",
	Short: "Archive one or more projects by moving them to the archive directory",
	Long: `Archive one or more projects by moving their directories to ~/Overlord/archive/<name>/ and
updating their status to 'archived' in the registry.

Name arguments can be project names or aliases.

Examples:
  overlord archive myproject              # Archive a single project
  overlord archive foo bar baz            # Archive multiple projects
  overlord archive myproject --yes        # Archive without confirmation`,
	Args: cobra.MinimumNArgs(1),
	RunE: runArchive,
}

func init() {
	rootCmd.AddCommand(archiveCmd)
	archiveCmd.Flags().BoolVarP(&archiveOpts.noConfirm, "yes", "y", false, "Skip confirmation prompt")
}

// resolvedProject holds pre-resolved info for a project to be archived.
type resolvedProject struct {
	name       string
	project    registry.Project
	sourcePath string
	destPath   string
}

func runArchive(cmd *cobra.Command, args []string) error {
	// Load registry
	reg, err := archiveLoadRegistry()
	if err != nil {
		return err
	}

	resolved, resolveErrors := resolveArchiveProjects(reg, args)

	s := NewStyles()

	// Print resolution errors
	printSkippedProjects(resolveErrors, s)

	if len(resolved) == 0 {
		return fmt.Errorf("no valid projects to archive")
	}

	// Confirm before archiving
	if !archiveOpts.noConfirm {
		confirmed, err := confirmArchiveMultiple(resolved)
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if !confirmed {
			fmt.Println("Cancelled")
			return nil
		}
	}

	// Archive each project
	var archiveErrors []string
	archiveSuccessCount := 0

	for _, rp := range resolved {
		err := archiveOneProject(reg, rp)
		if err != nil {
			archiveErrors = append(archiveErrors, fmt.Sprintf("'%s': %v", rp.name, err))
			continue
		}
		archiveSuccessCount++
	}

	// Print archive errors
	if len(archiveErrors) > 0 {
		fmt.Println()
		for _, e := range archiveErrors {
			fmt.Printf("%s %s\n", s.Error.Render("Failed"), e)
		}
	}

	// All archives failed — nothing to persist
	if archiveSuccessCount == 0 {
		return fmt.Errorf("all archive operations failed")
	}

	// At least one archive succeeded — persist registry changes
	if err := archiveSaveRegistry(reg); err != nil {
		fmt.Printf("%s Registry save failed after %d successful archive(s); filesystem and registry may be out of sync: %v\n",
			s.StrongWarn.Render("Warning:"), archiveSuccessCount, err)
		return err
	}

	return nil
}

// archiveOneProject archives a single pre-resolved project.
func archiveOneProject(reg *registry.Registry, rp resolvedProject) error {
	// Define styles for output
	s := NewStyles()
	labelStyle := s.Label
	successStyle := s.Success
	pathStyle := s.Path

	// Create archive directory parent
	archiveParent := filepath.Dir(rp.destPath)
	if err := os.MkdirAll(archiveParent, 0o755); err != nil {
		return fmt.Errorf("failed to create archive directory: %w", err)
	}

	if err := fireProjectEvent(lifecycle.ProjectArchiving, rp.name, rp.sourcePath); err != nil {
		return fmt.Errorf("pre-archive hook failed: %w", err)
	}

	// Move directory
	if err := fileutil.MoveDirectory(rp.sourcePath, rp.destPath); err != nil {
		return fmt.Errorf("failed to move directory: %w", err)
	}

	// Update registry
	archivePath := fmt.Sprintf("archive/%s", rp.name)
	project := rp.project
	project.Path = archivePath
	project.Status.State = registry.StateArchived
	reg.Projects[rp.name] = project

	postArchiveResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:        lifecycle.ProjectArchived,
		ProjectName: rp.name,
		ProjectPath: rp.destPath,
		DestPath:    rp.destPath,
		Settings:    nil,
	})

	// Print success message
	fmt.Printf("%s '%s'\n", successStyle.Render("Archived"), rp.name)
	fmt.Printf("  %s %s\n", labelStyle.Render("From:"), pathStyle.Render(fileutil.FormatPathWithTilde(rp.sourcePath)))
	fmt.Printf("  %s %s\n", labelStyle.Render("To:"), pathStyle.Render(fileutil.FormatPathWithTilde(rp.destPath)))
	fmt.Println()

	// Print any warnings from hooks
	if len(postArchiveResult.Warnings) > 0 {
		for _, warning := range postArchiveResult.Warnings {
			fmt.Printf("%s %s\n", s.StrongWarn.Render("Warning:"), warning)
		}
		fmt.Println()
	}

	return nil
}

func resolveArchiveProjects(reg *registry.Registry, args []string) ([]resolvedProject, []string) {
	// Resolve all projects upfront
	var resolved []resolvedProject
	var resolveErrors []string

	for _, name := range args {
		result, err := reg.ResolveOne(name)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': not found", name))
			continue
		}

		project := result.Project

		if project.Status.State == registry.StateArchived {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': already archived (location: %s)", result.Name, project.Path))
			continue
		}

		sourcePath, err := fileutil.ExpandProjectPath(project.Path, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand source path: %v", result.Name, err))
			continue
		}

		archivePath := fmt.Sprintf("archive/%s", result.Name)
		destPath, err := fileutil.ExpandProjectPath(archivePath, reg.Settings.BaseDir)
		if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to expand destination path: %v", result.Name, err))
			continue
		}

		if _, err := os.Stat(sourcePath); errors.Is(err, fs.ErrNotExist) {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': directory does not exist: %s", result.Name, sourcePath))
			continue
		} else if err != nil {
			resolveErrors = append(resolveErrors, fmt.Sprintf("'%s': failed to check directory: %v", result.Name, err))
			continue
		}

		resolved = append(resolved, resolvedProject{
			name:       result.Name,
			project:    project,
			sourcePath: sourcePath,
			destPath:   destPath,
		})
	}

	return resolved, resolveErrors
}
