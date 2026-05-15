package cmd

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/stack"
)

var (
	startManageLoadStackConfig = loadStackConfig
	startManageSaveStackConfig = saveStackConfig
	startManageLoadRegistry    = loadRegistry
	startManageDefaultServices = stack.DefaultServices
)

var startListCmd = &cobra.Command{
	Use:     "list",
	Short:   "List configured projects and services",
	Aliases: []string{"ls"},
	Args:    cobra.NoArgs,
	RunE:    runStartList,
}

var startAddCmd = &cobra.Command{
	Use:   "add <project...>",
	Short: "Add projects to startup list",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runStartAdd,
}

var startRemoveCmd = &cobra.Command{
	Use:     "remove <project...>",
	Short:   "Remove projects from startup list",
	Aliases: []string{"rm"},
	Args:    cobra.MinimumNArgs(1),
	RunE:    runStartRemove,
}

func init() {
	startCmd.AddCommand(startListCmd)
	startCmd.AddCommand(startAddCmd)
	startCmd.AddCommand(startRemoveCmd)
}

func runStartList(cmd *cobra.Command, args []string) error {
	cfg, err := startManageLoadStackConfig()
	if err != nil {
		return cmdError("start list", "failed to load stack config", err.Error())
	}

	s := NewStyles()
	w := cmd.ErrOrStderr()

	// Print configured projects.
	fmt.Fprintf(w, "  %s\n", s.Label.Render("Projects"))
	if len(cfg.Projects) == 0 {
		fmt.Fprintf(w, "  %s\n", s.Dim.Render("No projects configured. Use 'overlord start add <project>' to add projects."))
	} else {
		for i, name := range cfg.Projects {
			fmt.Fprintf(w, "  %s %s\n", s.Dim.Render(fmt.Sprintf("%d.", i+1)), s.Bold.Render(name))
		}
	}
	fmt.Fprintln(w)

	// Print enabled services with their settings.
	fmt.Fprintf(w, "  %s\n", s.Label.Render("Services"))
	services := startManageDefaultServices(cfg)
	if len(services) == 0 {
		fmt.Fprintf(w, "  %s\n", s.Dim.Render("No services enabled."))
	} else {
		for _, svc := range services {
			svcCfg := cfg.Services[svc.Name()]
			fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render(svc.WindowName()), s.Success.Render("enabled"))
			if len(svcCfg.Settings) > 0 {
				keys := make([]string, 0, len(svcCfg.Settings))
				for k := range svcCfg.Settings {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					fmt.Fprintf(w, "    %-12s %s\n", s.Dim.Render(k+":"), s.Value.Render(svcCfg.Settings[k]))
				}
			}
		}
	}
	fmt.Fprintln(w)

	return nil
}

func runStartAdd(cmd *cobra.Command, args []string) error {
	cfg, err := startManageLoadStackConfig()
	if err != nil {
		return cmdError("start add", "failed to load stack config", err.Error())
	}

	reg, err := startManageLoadRegistry()
	if err != nil {
		return cmdError("start add", "failed to load registry", err.Error())
	}

	s := NewStyles()
	w := cmd.ErrOrStderr()

	// Build a set of names already in the project list for duplicate detection.
	existing := make(map[string]struct{}, len(cfg.Projects))
	for _, name := range cfg.Projects {
		existing[name] = struct{}{}
	}

	added := 0
	for _, arg := range args {
		res, err := reg.ResolveOne(arg)
		if err != nil {
			fmt.Fprintf(w, "%s could not resolve project %q: %v\n",
				s.Warn.Render("Warning:"), arg, err)
			continue
		}

		if res.Project.Status.State == registry.StateArchived {
			fmt.Fprintf(w, "%s Skipping %q: project is archived\n",
				s.Warn.Render("Warning:"), res.Name)
			continue
		}

		resolved := res.Name

		if _, ok := existing[resolved]; ok {
			fmt.Fprintf(w, "%s project %q is already in the startup list\n",
				s.Warn.Render("Warning:"), resolved)
			continue
		}

		cfg.Projects = append(cfg.Projects, resolved)
		existing[resolved] = struct{}{}
		fmt.Fprintf(w, "%s added %s to startup list\n",
			s.Success.Render("Added:"), s.Bold.Render(resolved))
		added++
	}

	if added == 0 {
		return nil
	}

	if err := startManageSaveStackConfig(cfg); err != nil {
		return cmdError("start add", "failed to save stack config", err.Error())
	}

	return nil
}

func runStartRemove(cmd *cobra.Command, args []string) error {
	cfg, err := startManageLoadStackConfig()
	if err != nil {
		return cmdError("start remove", "failed to load stack config", err.Error())
	}

	s := NewStyles()
	w := cmd.ErrOrStderr()

	removed := 0
	for _, arg := range args {
		idx := -1
		for i, name := range cfg.Projects {
			if name == arg {
				idx = i
				break
			}
		}

		if idx == -1 {
			fmt.Fprintf(w, "%s project %q is not in the startup list\n",
				s.Warn.Render("Warning:"), arg)
			continue
		}

		cfg.Projects = append(cfg.Projects[:idx], cfg.Projects[idx+1:]...)
		fmt.Fprintf(w, "%s removed %s from startup list\n",
			s.Success.Render("Removed:"), s.Bold.Render(arg))
		removed++
	}

	if removed == 0 {
		return nil
	}

	if err := startManageSaveStackConfig(cfg); err != nil {
		return cmdError("start remove", "failed to save stack config", err.Error())
	}

	return nil
}
