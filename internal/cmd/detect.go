package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
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

	baseDir := reg.Settings.BaseDir
	expandedBase, err := registry.ExpandPath(baseDir)
	if err != nil {
		return fmt.Errorf("failed to expand base dir: %w", err)
	}

	// Build set of known paths (relative to base dir)
	knownPaths := buildKnownPaths(reg, expandedBase)

	// Build scan targets: category paths + archive
	type scanTarget struct {
		dir      string // relative dir to scan under base
		category string // inferred category label
	}

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
	if detectCategoryOpt != "" {
		var filtered []scanTarget
		for _, t := range targets {
			if t.category == detectCategoryOpt {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("unknown category: %s\nValid categories: %s, archive",
				detectCategoryOpt, strings.Join(validCategoryNames(), ", "))
		}
		targets = filtered
	}

	// Scan each target directory for unregistered projects
	var unregistered []unregisteredProject
	for _, target := range targets {
		absDir := filepath.Join(expandedBase, target.dir)
		entries, err := os.ReadDir(absDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to read directory %s: %w", absDir, err)
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

	if detectJSON {
		return outputDetectJSON(unregistered)
	}
	return outputDetectHuman(unregistered)
}

func buildKnownPaths(reg *registry.Registry, expandedBase string) map[string]struct{} {
	known := make(map[string]struct{})
	for _, project := range reg.Projects {
		p := project.Path

		// Handle absolute paths: convert to relative
		if filepath.IsAbs(p) {
			rel, err := filepath.Rel(expandedBase, p)
			if err == nil {
				p = rel
			}
		}

		// Handle ~ prefix paths
		if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~") {
			expanded, err := registry.ExpandPath(p)
			if err == nil {
				rel, err := filepath.Rel(expandedBase, expanded)
				if err == nil {
					p = rel
				}
			}
		}

		known[p] = struct{}{}
	}
	return known
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
