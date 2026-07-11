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

	services, preflightWarnings := preflightServices(cfg, DefaultServices(cfg))

	if exists {
		result := &StartResult{Warnings: preflightWarnings}
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

	warnings := append([]string{}, preflightWarnings...)
	sessionCreated := false
	cleanupOnError := func(cause error) error {
		if !sessionCreated {
			return cause
		}
		if killErr := session.Kill(); killErr != nil {
			return fmt.Errorf("%w (rollback failed: %v)", cause, killErr)
		}
		return cause
	}

	for i, svc := range services {
		dir := svc.WorkDir()
		if dir == "" {
			dir, _ = os.UserHomeDir()
		}

		cmd := buildCommand(svc)

		if i == 0 {
			// First service: create the session with a named window.
			session.Dir = dir
			if err := session.CreateWithCommand(svc.WindowName(), cmd); err != nil {
				return nil, fmt.Errorf("creating session with %s: %w", svc.Name(), err)
			}
			sessionCreated = true
		} else {
			// Subsequent services: add a new window.
			origDir := session.Dir
			session.Dir = dir
			if err := session.NewWindow(svc.WindowName(), cmd); err != nil {
				return nil, cleanupOnError(fmt.Errorf("creating window for %s: %w", svc.Name(), err))
			}
			session.Dir = origDir
		}

		// If the service defines a pane command, split the window vertically.
		if paneCmd := svc.PaneCommand(); paneCmd != "" {
			origDir := session.Dir
			session.Dir = dir
			if err := session.SplitWindow(svc.WindowName(), paneCmd); err != nil {
				warnings = append(warnings, fmt.Sprintf("split pane for %s: %v", svc.Name(), err))
			}
			session.Dir = origDir
		}
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
