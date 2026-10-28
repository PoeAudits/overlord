package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
)

var (
	detectJSON        bool
	detectCategoryOpt string
)

// detectCmd represents the detect command
var detectCmd = &cobra.Command{
	Use:   "detect",
	Short: "Find projects on disk not in the registry",
	Long: `Scan the Overlord directory structure for project directories that exist
on disk but are not registered in the registry.

Scans all category directories (projects/*, core/*, sandbox/) and the archive/
directory, comparing against registered project paths.

Examples:
  overlord detect
  overlord detect --category libs
  overlord detect --json`,
	RunE: runDetect,
}

func init() {
	rootCmd.AddCommand(detectCmd)
	detectCmd.Flags().BoolVar(&detectJSON, "json", false, "Output in JSON format")
	detectCmd.Flags().StringVar(&detectCategoryOpt, "category", "", "Filter by category")
}

type unregisteredProject struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Category string `json:"category"`
}

func runDetect(cmd *cobra.Command, args []string) error {
	reg, err := loadRegistry()
	if err != nil {
		return err
	}

	expandedBase, err := expandBaseDir(reg)
	if err != nil {
		return err
	}

	// Build set of known paths (relative to base dir)
	knownPaths := buildKnownPaths(reg, expandedBase)

	targets, err := detectionTargets(detectCategoryOpt)
	if err != nil {
		return err
	}
	unregistered, err := scanUnregisteredProjects(expandedBase, knownPaths, targets)
	if err != nil {
		return err
	}

	if detectJSON {
		return outputDetectJSON(unregistered)
	}
	return outputDetectHuman(unregistered)
}

func buildKnownPaths(reg *registry.Registry, expandedBase string) map[string]struct{} {
	known := make(map[string]struct{})
	for _, project := range reg.Projects {
		known[normalizeKnownPath(project.Path, expandedBase)] = struct{}{}
	}
	return known
}

func normalizeKnownPath(path, expandedBase string) string {
	if filepath.IsAbs(path) {
		rel, err := filepath.Rel(expandedBase, path)
		if err == nil {
			path = rel
		}
	}

	if !strings.HasPrefix(path, "~") {
		return path
	}

	expanded, err := registry.ExpandPath(path)
	if err != nil {
		return path
	}

	rel, err := filepath.Rel(expandedBase, expanded)
	if err != nil {
		return path
	}

	return rel
}

func outputDetectHuman(projects []unregisteredProject) error {
	if len(projects) == 0 {
		fmt.Println("All projects are registered.")
		return nil
	}

	s := NewStyles()
	headerStyle := s.Label
	pathStyle := s.Name
	categoryStyle := s.Category
	countStyle := s.Warn

	fmt.Printf("%s %s\n\n",
		headerStyle.Render("Unregistered projects:"),
		countStyle.Render(fmt.Sprintf("(%d found)", len(projects))))

	for _, p := range projects {
		fmt.Printf("  %s  %s\n",
			pathStyle.Render(p.Path),
			categoryStyle.Render("("+p.Category+")"))
	}

	fmt.Printf("\nUse %s to register a project.\n",
		headerStyle.Render("overlord add <name> <path>"))
	return nil
}

func outputDetectJSON(projects []unregisteredProject) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(projects); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func validCategoryNames() []string {
	cats := registry.ValidCategories()
	names := make([]string, len(cats))
	for i, c := range cats {
		names[i] = string(c)
	}
	return names
}

type scanTarget struct {
	dir      string // relative dir to scan under base
	category string // inferred category label
}

func detectionTargets(category string) ([]scanTarget, error) {
	// Build scan targets: category paths + archive
	var targets []scanTarget
	for _, cat := range registry.ValidCategories() {
		catPath := strings.TrimSuffix(cat.Path(), "/")
		if catPath == "" {
			continue
		}
		targets = append(targets, scanTarget{dir: catPath, category: string(cat)})
	}
	targets = append(targets, scanTarget{dir: "archive", category: "archive"})

	// Filter by category if specified
	if category != "" {
		var filtered []scanTarget
		for _, t := range targets {
			if t.category == category {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			return nil, cmdError("detect", fmt.Sprintf("unknown category: %s", category),
				fmt.Sprintf("Valid categories: %s, archive", strings.Join(validCategoryNames(), ", ")))
		}
		targets = filtered
	}

	return targets, nil
}

func scanUnregisteredProjects(expandedBase string, knownPaths map[string]struct{}, targets []scanTarget) ([]unregisteredProject, error) {
	// Scan each target directory for unregistered projects
	var unregistered []unregisteredProject
	for _, target := range targets {
		absDir := filepath.Join(expandedBase, target.dir)
		entries, err := os.ReadDir(absDir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("failed to read directory %s: %w", absDir, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			relPath := filepath.Join(target.dir, entry.Name())
			if _, known := knownPaths[relPath]; !known {
				unregistered = append(unregistered, unregisteredProject{
					Name:     entry.Name(),
					Path:     relPath,
					Category: target.category,
				})
			}
		}
	}

	return unregistered, nil
}
