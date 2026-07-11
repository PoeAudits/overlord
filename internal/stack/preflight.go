package stack

import (
	"fmt"
	"strings"
)

func preflightServices(cfg *StackConfig, services []Service) ([]Service, []string) {
	if !serviceEnabled(cfg, "opencode") || !serviceEnabled(cfg, "opencode-desktop") {
		return services, nil
	}

	cliVersion, cliPkg, ok := installedOpenCodePackageVersion([]string{"opencode-bin", "opencode"})
	if !ok {
		return services, nil
	}
	desktopVersion, desktopPkg, ok := installedOpenCodePackageVersion([]string{"opencode-desktop-bin", "opencode-desktop", "opencode-desktop-app"})
	if !ok {
		return services, nil
	}

	if upstreamVersion(cliVersion) == upstreamVersion(desktopVersion) {
		return services, nil
	}

	warning := fmt.Sprintf(
		"skipping OpenCodeDesktop: %s is %s but %s is %s; update both packages together",
		desktopPkg,
		desktopVersion,
		cliPkg,
		cliVersion,
	)

	filtered := services[:0]
	for _, svc := range services {
		if svc.Name() == "opencode-desktop" {
			continue
		}
		filtered = append(filtered, svc)
	}
	return filtered, []string{warning}
}

func serviceEnabled(cfg *StackConfig, name string) bool {
	if cfg == nil || cfg.Services == nil {
		return false
	}
	return cfg.Services[name].Enabled
}

func installedOpenCodePackageVersion(names []string) (version string, packageName string, ok bool) {
	packageQuery := "yay"
	if _, err := commandLookPath(packageQuery); err != nil {
		packageQuery = "pacman"
	}
	if _, err := commandLookPath(packageQuery); err != nil {
		return "", "", false
	}
	for _, name := range names {
		out, err := commandExec(packageQuery, "-Q", name).Output()
		if err != nil {
			continue
		}
		fields := strings.Fields(string(out))
		if len(fields) >= 2 {
			return fields[1], fields[0], true
		}
	}
	return "", "", false
}

func upstreamVersion(version string) string {
	idx := strings.LastIndex(version, "-")
	if idx == -1 {
		return version
	}
	return version[:idx]
}
