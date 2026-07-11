package stack

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPreflightServices_SkipsDesktopOnVersionMismatch(t *testing.T) {
	withPackageVersions(t, map[string]string{
		"opencode-bin":     "opencode-bin 1.17.3-1",
		"opencode-desktop": "opencode-desktop 1.16.2-1",
	})

	services, warnings := preflightServices(enabledConfig("opencode", "opencode-desktop"), []Service{
		&stubService{name: "opencode"},
		&stubService{name: "opencode-desktop"},
	})

	if len(services) != 1 || services[0].Name() != "opencode" {
		t.Fatalf("expected only opencode service, got %v", serviceNames(services))
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "skipping OpenCodeDesktop") {
		t.Fatalf("expected mismatch warning, got %v", warnings)
	}
}

func TestPreflightServices_AllowsDesktopWhenUpstreamVersionsMatch(t *testing.T) {
	withPackageVersions(t, map[string]string{
		"opencode-bin":         "opencode-bin 1.17.3-1",
		"opencode-desktop-bin": "opencode-desktop-bin 1.17.3-2",
	})

	services, warnings := preflightServices(enabledConfig("opencode", "opencode-desktop"), []Service{
		&stubService{name: "opencode"},
		&stubService{name: "opencode-desktop"},
	})

	if len(services) != 2 {
		t.Fatalf("expected both services, got %v", serviceNames(services))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestPreflightServices_AllowsDesktopAppPackageName(t *testing.T) {
	withPackageVersions(t, map[string]string{
		"opencode-bin":        "opencode-bin 1.17.3-1",
		"opencode-desktop-app": "opencode-desktop-app 1.17.3-4",
	})

	services, warnings := preflightServices(enabledConfig("opencode", "opencode-desktop"), []Service{
		&stubService{name: "opencode"},
		&stubService{name: "opencode-desktop"},
	})

	if len(services) != 2 {
		t.Fatalf("expected both services, got %v", serviceNames(services))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func TestPreflightServices_DoesNotBlockWhenPackageVersionsUnavailable(t *testing.T) {
	oldLookPath := commandLookPath
	commandLookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	t.Cleanup(func() { commandLookPath = oldLookPath })

	services, warnings := preflightServices(enabledConfig("opencode", "opencode-desktop"), []Service{
		&stubService{name: "opencode"},
		&stubService{name: "opencode-desktop"},
	})

	if len(services) != 2 {
		t.Fatalf("expected both services, got %v", serviceNames(services))
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
}

func withPackageVersions(t *testing.T, outputs map[string]string) {
	t.Helper()
	oldExec := commandExec
	oldLookPath := commandLookPath
	commandLookPath = func(string) (string, error) { return "/usr/bin/yay", nil }
	commandExec = func(name string, args ...string) *exec.Cmd {
		if name == "yay" && len(args) == 2 && args[0] == "-Q" {
			if output, ok := outputs[args[1]]; ok {
				return commandWithOutput(output)
			}
		}
		return commandWithOutputAndExit("", 1)
	}
	t.Cleanup(func() {
		commandExec = oldExec
		commandLookPath = oldLookPath
	})
}

func serviceNames(services []Service) []string {
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.Name())
	}
	return names
}
