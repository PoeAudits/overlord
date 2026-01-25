package registry_test

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/registry"
)

func ExampleRegistry_Resolve_exactName() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"overlord-v2": {
				Path:        "core/tools/overlord-v2",
				Category:    registry.CategoryCoreTools,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-15",
				Description: "Project management CLI tool",
				Aliases:     []string{"overlord", "ol"},
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	results := reg.Resolve("overlord-v2")
	if len(results) > 0 {
		fmt.Printf("Found: %s (score: %d, matched on: %s)\n",
			results[0].Name, results[0].Score, results[0].MatchedOn)
	}

	// Output:
	// Found: overlord-v2 (score: 1000, matched on: name)
}

func ExampleRegistry_Resolve_exactAlias() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"overlord-v2": {
				Path:        "core/tools/overlord-v2",
				Category:    registry.CategoryCoreTools,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-15",
				Description: "Project management CLI tool",
				Aliases:     []string{"overlord", "ol"},
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	results := reg.Resolve("overlord")
	if len(results) > 0 {
		fmt.Printf("Found: %s (score: %d, matched on: %s)\n",
			results[0].Name, results[0].Score, results[0].MatchedOn)
	}

	// Output:
	// Found: overlord-v2 (score: 900, matched on: alias:overlord)
}

func ExampleRegistry_Resolve_fuzzy() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"overlord-v2": {
				Path:        "core/tools/overlord-v2",
				Category:    registry.CategoryCoreTools,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-15",
				Description: "Project management CLI tool",
				Aliases:     []string{"overlord", "ol"},
				Status:      registry.Status{State: registry.StateActive},
			},
			"web-dashboard": {
				Path:        "projects/web/dashboard",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageTypeScript,
				Created:     "2024-02-01",
				Description: "Admin dashboard",
				Aliases:     []string{"dashboard"},
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	// Fuzzy match with typo
	results := reg.Resolve("overlrd")
	if len(results) > 0 {
		fmt.Printf("Found %d match(es)\n", len(results))
		fmt.Printf("Best match: %s\n", results[0].Name)
	}

	// Output:
	// Found 1 match(es)
	// Best match: overlord-v2
}

func ExampleRegistry_ResolveOne_success() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"overlord-v2": {
				Path:        "core/tools/overlord-v2",
				Category:    registry.CategoryCoreTools,
				Lang:        registry.LanguageGo,
				Created:     "2024-01-15",
				Description: "Project management CLI tool",
				Aliases:     []string{"overlord", "ol"},
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	result, err := reg.ResolveOne("overlord")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Resolved to: %s\n", result.Name)
	fmt.Printf("Description: %s\n", result.Project.Description)

	// Output:
	// Resolved to: overlord-v2
	// Description: Project management CLI tool
}

func ExampleRegistry_ResolveOne_notFound() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{},
	}

	_, err := reg.ResolveOne("nonexistent")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Output:
	// Error: no project found matching 'nonexistent'
}

func ExampleRegistry_Resolve_multipleMatches() {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "/home/user/projects",
			ThoughtsDir: "/home/user/thoughts",
		},
		Projects: map[string]registry.Project{
			"service-gateway": {
				Path:        "projects/services/gateway",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2024-02-01",
				Description: "API gateway service",
				Status:      registry.Status{State: registry.StateActive},
			},
			"service-auth": {
				Path:        "projects/services/auth",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2024-02-15",
				Description: "Auth service",
				Status:      registry.Status{State: registry.StateActive},
			},
			"service-users": {
				Path:        "projects/services/users",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2024-03-01",
				Description: "Users service",
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	// Search for "service" - should match all three projects
	results := reg.Resolve("service")
	fmt.Printf("Found %d matches for 'service'\n", len(results))

	// Output:
	// Found 3 matches for 'service'
}
