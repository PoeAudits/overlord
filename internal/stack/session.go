package stack

import (
	"fmt"
	"os"
	"strings"

	"github.com/PoeAudits/overlord/internal/lifecycle"
	"github.com/PoeAudits/overlord/internal/tmux"
)

// SessionName is the tmux session name for the development stack.
const SessionName = "StartServer"

// ServiceStatus represents the health status of a service.
type ServiceStatus string

const (
	StatusStarted   ServiceStatus = "started"
	StatusHealthy   ServiceStatus = "healthy"
	StatusUnhealthy ServiceStatus = "unhealthy"
	StatusSkipped   ServiceStatus = "skipped"
)

// StartResult holds the outcome of a StartStack invocation.
type StartResult struct {
	ServiceResults []ServiceResult
	Warnings       []string
}

// ServiceResult records the outcome for a single service.
type ServiceResult struct {
	Name        string
	DisplayName string
	Status      ServiceStatus
	Error       error
}

// StartStack orchestrates launching all enabled services in a tmux session
// called "StartServer". If the session already exists, all services are
// reported as "skipped" (design decision D1: idempotent, no --force flag).
func StartStack(cfg *StackConfig, opts ...tmux.Option) (*StartResult, error) {
	session := tmux.NewSession(SessionName, "", opts...)

	exists, err := session.Exists()
	if err != nil {
		return nil, fmt.Errorf("checking session existence: %w", err)
	}

	services := DefaultServices(cfg)

	if exists {
		result := &StartResult{}
		for _, svc := range services {
			result.ServiceResults = append(result.ServiceResults, ServiceResult{
				Name:        svc.Name(),
				DisplayName: svc.WindowName(),
				Status:      StatusSkipped,
			})
		}
		return result, nil
	}

	hookResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:      lifecycle.StackStarting,
		StackName: SessionName,
	})
	if hookResult.Err != nil {
		return nil, fmt.Errorf("pre-hook failed: %w", hookResult.Err)
	}

	if len(services) == 0 {
		return &StartResult{}, nil
	}

	warnings, err := launchServices(session, services)
	if err != nil {
		return nil, err
	}

	results := make([]ServiceResult, len(services))
	for i, svc := range services {
		results[i] = ServiceResult{
			Name:        svc.Name(),
			DisplayName: svc.WindowName(),
			Status:      StatusStarted,
		}
	}

	startResult := &StartResult{
		ServiceResults: results,
		Warnings:       warnings,
	}

	postResult := lifecycle.DefaultBus.Fire(lifecycle.Event{
		Type:      lifecycle.StackStarted,
		StackName: SessionName,
	})
	if postResult.Err != nil {
		startResult.Warnings = append(startResult.Warnings, postResult.Err.Error())
	}
	startResult.Warnings = append(startResult.Warnings, postResult.Warnings...)

	return startResult, nil
}

// buildCommand composes the full shell command for a service, prepending
// any environment variables using the env command.
func buildCommand(svc Service) string {
	envVars := svc.Env()
	cmd := svc.Command()
	if len(envVars) == 0 {
		return cmd
	}
	return "env " + strings.Join(envVars, " ") + " " + cmd
}

// launchServices rolls back the session only after its initial window was created.
func launchServices(session *tmux.Session, services []Service) ([]string, error) {
	var warnings []string
	for i, svc := range services {
		warning, err := launchServiceWindow(session, svc, i == 0)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			if killErr := session.Kill(); killErr != nil {
				return nil, fmt.Errorf("%w (rollback failed: %v)", err, killErr)
			}
			return nil, err
		}
		if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings, nil
}

// launchServiceWindow shares directory selection and optional panes between start and restart.
func launchServiceWindow(session *tmux.Session, svc Service, first bool) (string, error) {
	originalDir := session.Dir
	defer func() { session.Dir = originalDir }()
	session.Dir = svc.WorkDir()
	if session.Dir == "" {
		session.Dir, _ = os.UserHomeDir()
	}
	create := session.NewWindow
	operation := "creating window for"
	if first {
		create = session.CreateWithCommand
		operation = "creating session with"
	}
	if err := create(svc.WindowName(), buildCommand(svc)); err != nil {
		return "", fmt.Errorf("%s %s: %w", operation, svc.Name(), err)
	}
	if paneCmd := svc.PaneCommand(); paneCmd != "" {
		if err := session.SplitWindow(svc.WindowName(), paneCmd); err != nil {
			return fmt.Sprintf("split pane for %s: %v", svc.Name(), err), nil
		}
	}
	return "", nil
}
