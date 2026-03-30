package cmd

import (
	"reflect"
	"testing"

	"github.com/PoeAudits/overlord/internal/registry"
)

func TestResolveProjectTargets(t *testing.T) {
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

	tests := []struct {
		name string
		args []string
		all  bool
		want []string
	}{
		{
			name: "all returns sorted names",
			all:  true,
			want: []string{"api", "app"},
		},
		{
			name: "dedupes aliases",
			args: []string{"app", "application"},
			want: []string{"app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			names, err := resolveProjectTargets(reg, tt.args, tt.all)
			if err != nil {
				t.Fatalf("resolveProjectTargets() error = %v", err)
			}

			if !reflect.DeepEqual(names, tt.want) {
				t.Fatalf("resolveProjectTargets() = %v, want %v", names, tt.want)
			}
		})
	}
}
