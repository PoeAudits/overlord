package cmd

import (
	"reflect"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestResolveActivateTargets(t *testing.T) {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"app": {
				Path:        "projects/web/app",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageGo,
				Created:     "2026-01-01",
				Description: "app",
				Aliases:     []string{"application"},
				Status:      registry.Status{State: registry.StateActive},
			},
			"api": {
				Path:        "projects/services/api",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2026-01-01",
				Description: "api",
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	t.Run("all returns sorted names", func(t *testing.T) {
		names, err := resolveActivateTargets(reg, nil, true)
		if err != nil {
			t.Fatalf("resolveActivateTargets() error = %v", err)
		}

		want := []string{"api", "app"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("resolveActivateTargets() = %v, want %v", names, want)
		}
	})

	t.Run("dedupes aliases", func(t *testing.T) {
		names, err := resolveActivateTargets(reg, []string{"app", "application"}, false)
		if err != nil {
			t.Fatalf("resolveActivateTargets() error = %v", err)
		}

		want := []string{"app"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("resolveActivateTargets() = %v, want %v", names, want)
		}
	})
}

func TestResolveDeactivateTargets(t *testing.T) {
	reg := &registry.Registry{
		Version: 2,
		Settings: registry.Settings{
			BaseDir:     "~/Overlord",
			ThoughtsDir: "~/thoughts",
		},
		Projects: map[string]registry.Project{
			"worker": {
				Path:        "projects/services/worker",
				Category:    registry.CategoryServices,
				Lang:        registry.LanguageGo,
				Created:     "2026-01-01",
				Description: "worker",
				Aliases:     []string{"wk"},
				Status:      registry.Status{State: registry.StateActive},
			},
			"frontend": {
				Path:        "projects/web/frontend",
				Category:    registry.CategoryWeb,
				Lang:        registry.LanguageTypeScript,
				Created:     "2026-01-01",
				Description: "frontend",
				Status:      registry.Status{State: registry.StateActive},
			},
		},
	}

	t.Run("all returns sorted names", func(t *testing.T) {
		names, err := resolveDeactivateTargets(reg, nil, true)
		if err != nil {
			t.Fatalf("resolveDeactivateTargets() error = %v", err)
		}

		want := []string{"frontend", "worker"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("resolveDeactivateTargets() = %v, want %v", names, want)
		}
	})

	t.Run("dedupes aliases", func(t *testing.T) {
		names, err := resolveDeactivateTargets(reg, []string{"worker", "wk"}, false)
		if err != nil {
			t.Fatalf("resolveDeactivateTargets() error = %v", err)
		}

		want := []string{"worker"}
		if !reflect.DeepEqual(names, want) {
			t.Fatalf("resolveDeactivateTargets() = %v, want %v", names, want)
		}
	})
}
