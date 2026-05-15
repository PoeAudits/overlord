package cmd

import "github.com/PoeAudits/overlord/internal/registry"

func testRegistry(projects map[string]registry.Project) *registry.Registry {
	return &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir: "~/Overlord",
		},
		Projects: projects,
	}
}

func testProject(path string, category registry.Category, lang registry.Language, state registry.State, aliases ...string) registry.Project {
	return registry.Project{
		Path:        path,
		Category:    category,
		Lang:        lang,
		Created:     "2024-01-15",
		Description: "test project",
		Aliases:     aliases,
		Status: registry.Status{
			State: state,
		},
	}
}
