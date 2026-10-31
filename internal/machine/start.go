package machine

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/PoeAudits/overlord/internal/config"
	"github.com/PoeAudits/overlord/internal/system"
)

// ServiceStatus is what Start found or did for one service.
type ServiceStatus string

const (
	// Healthy: the service already answered; nothing was launched.
	Healthy ServiceStatus = "ok"
	// Started: the service was launched and became healthy, or the app was
	// launched.
	Started ServiceStatus = "started"
	// Skipped: the service is disabled in the config.
	Skipped ServiceStatus = "skipped"
	// Failed: the launch failed or health did not arrive in time.
	Failed ServiceStatus = "failed"
)

// ServiceResult is the outcome of Start for one service or the app.
type ServiceResult struct {
	Name   string
	Status ServiceStatus
	Detail string
	// Log is the file a failed service's output went to, if it was launched.
	Log string
}

// AppName names the T3 Code desktop app in Start's results.
const AppName = "T3 Code app"

// Start brings up every enabled service that is not healthy, in order, and
// then the T3 Code desktop app. A service that fails does not stop the rest.
func Start(ctx context.Context, sys system.System, services []config.Service) []ServiceResult {
	logs := filepath.Join(sys.Home, ".local", "state", "overlord")
	var results []ServiceResult
	for _, service := range services {
		results = append(results, startService(ctx, sys, logs, service))
	}
	return append(results, startApp(ctx, sys, logs))
}

// healthPoll is how often Start checks a launched service for health.
const healthPoll = 500 * time.Millisecond

func startService(ctx context.Context, sys system.System, logs string, service config.Service) ServiceResult {
	if !service.IsEnabled() {
		return ServiceResult{Name: service.Name, Status: Skipped, Detail: "disabled"}
	}
	if sys.Health.Healthy(ctx, service.Health) {
		return ServiceResult{Name: service.Name, Status: Healthy, Detail: "healthy"}
	}
	log, err := launch(sys, logs, service.Name, "sh", "-c", service.Command)
	if err != nil {
		return ServiceResult{Name: service.Name, Status: Failed, Detail: err.Error()}
	}
	for range max(1, int(service.Timeout/healthPoll)) {
		sys.Sleep(healthPoll)
		if sys.Health.Healthy(ctx, service.Health) {
			return ServiceResult{Name: service.Name, Status: Started, Detail: "healthy"}
		}
	}
	return ServiceResult{Name: service.Name, Status: Failed, Detail: "not healthy after " + service.Timeout.String(), Log: log}
}

// startApp launches the T3 Code desktop app through its desktop entry unless
// it is already running.
func startApp(ctx context.Context, sys system.System, logs string) ServiceResult {
	if appRunning(ctx, sys) {
		return ServiceResult{Name: AppName, Status: Healthy, Detail: "running"}
	}
	if _, err := launch(sys, logs, "t3-app", "gtk-launch", "t3code"); err != nil {
		return ServiceResult{Name: AppName, Status: Failed, Detail: err.Error()}
	}
	return ServiceResult{Name: AppName, Status: Started, Detail: "launched"}
}

// launch starts name detached with its output appended to logs/<logName>.log
// and returns that log's path.
func launch(sys system.System, logs, logName, name string, args ...string) (string, error) {
	if err := os.MkdirAll(logs, 0o755); err != nil {
		return "", err
	}
	log := filepath.Join(logs, logName+".log")
	return log, sys.Runner.Launch(sys.Home, log, name, args...)
}

// appRunning looks for the desktop app's main process: the t3code binary
// with no arguments. Its helper processes and the T3 server it spawns all
// carry arguments, so they do not match.
func appRunning(ctx context.Context, sys system.System) bool {
	_, err := sys.Runner.Run(ctx, sys.Home, "pgrep", "-f", "-x", "[^ ]*/t3code")
	return err == nil
}
