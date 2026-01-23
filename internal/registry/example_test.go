package registry_test

import (
	"fmt"
	"log"
	"path/filepath"

	"overlord-v2/internal/registry"
)

// Example demonstrates basic usage of Load and Save functions
func Example() {
	// Use temp directory for example
	tempDir := "/tmp/overlord-example"
	registryPath := filepath.Join(tempDir, "registry.yaml")

	// Load registry (creates default if not exists)
	reg, err := registry.Load(registryPath)
	if err != nil {
		log.Fatal(err)
	}

	// Add a project
	reg.Projects["my-cli-tool"] = registry.Project{
		Path:        "~/Overlord/my-cli-tool",
		Category:    registry.CategoryCLI,
		Lang:        registry.LanguageGo,
		Created:     "2024-01-15",
		Description: "A useful CLI tool",
		Aliases:     []string{"cli", "tool"},
		Tags:        []string{"productivity", "automation"},
		Status:      registry.Status{State: registry.StateActive},
	}

	// Save registry (creates backup automatically)
	if err := registry.Save(registryPath, reg); err != nil {
		log.Fatal(err)
	}

	// Load again to verify
	loaded, err := registry.Load(registryPath)
	if err != nil {
		log.Fatal(err)
	}

	project, ok := loaded.GetProject("my-cli-tool")
	if ok {
		fmt.Printf("Project: %s\n", project.Description)
		fmt.Printf("Category: %s\n", project.Category)
		fmt.Printf("Language: %s\n", project.Lang)
	}

	// Output:
	// Project: A useful CLI tool
	// Category: cli
	// Language: go
}
