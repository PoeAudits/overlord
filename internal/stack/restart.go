package stack

import (
	"fmt"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/tmux"
)

const StatusRestarted ServiceStatus = "restarted"

// RestartServices restarts named enabled services in the existing stack session.
func RestartServices(cfg *StackConfig, names []string, opts ...tmux.Option) (*StartResult, error) {
	session := tmux.NewSession(SessionName, "", opts...)
	exists, err := session.Exists()
	if err != nil {
		return nil, fmt.Errorf("checking session existence: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("tmux session %q: %w", SessionName, tmux.ErrSessionNotFound)
	}

	services := DefaultServices(cfg)
	selected, err := resolveServices(services, names)
	if err != nil {
		return nil, err
	}

	results := make([]ServiceResult, 0, len(selected))
	var warnings []string
	for _, svc := range selected {
		dir := svc.WorkDir()
		if dir == "" {
			dir, _ = os.UserHomeDir()
		}

		if err := session.KillWindow(svc.WindowName()); err != nil {
			// A missing service window in an otherwise-running session is the
			// common "newly configured service" case: just create it instead
			// of tearing down. Other kill-window errors still surface.
			if !tmux.IsWindowNotFound(err) {
				return nil, fmt.Errorf("killing window for %s: %w", svc.Name(), err)
			}
		}

		origDir := session.Dir
		session.Dir = dir
		if err := session.NewWindow(svc.WindowName(), buildCommand(svc)); err != nil {
			session.Dir = origDir
			return nil, fmt.Errorf("creating window for %s: %w", svc.Name(), err)
		}

		if paneCmd := svc.PaneCommand(); paneCmd != "" {
			if err := session.SplitWindow(svc.WindowName(), paneCmd); err != nil {
				warnings = append(warnings, fmt.Sprintf("split pane for %s: %v", svc.Name(), err))
			}
		}
		session.Dir = origDir

		results = append(results, ServiceResult{
			Name:        svc.Name(),
			DisplayName: svc.WindowName(),
			Status:      StatusRestarted,
		})
	}

	return &StartResult{ServiceResults: results, Warnings: warnings}, nil
}

func resolveServices(services []Service, names []string) ([]Service, error) {
	byName := make(map[string]Service, len(services)*2)
	for _, svc := range services {
		byName[normalizeServiceName(svc.Name())] = svc
		byName[normalizeServiceName(svc.WindowName())] = svc
	}

	selected := make([]Service, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		svc, ok := byName[normalizeServiceName(name)]
		if !ok {
			return nil, fmt.Errorf("unknown or disabled service %q", name)
		}
		key := svc.Name()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, svc)
	}
	return selected, nil
}

func normalizeServiceName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
