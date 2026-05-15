package stack

import "strconv"

const (
	t3CodeServiceName = "t3code"

	openCodeHostKey  = "host"
	openCodePortKey  = "port"
	t3CodeHostKey    = "host"
	t3CodePortKey    = "port"
	t3CodeWorkDirKey = "workdir"
	t3CodeBaseDirKey = "base_dir"
	t3CodeCommandKey = "command"

	frontEndPortKey    = "port"
	frontEndWorkDirKey = "workdir"

	tailscaleTargetKey = "target"

	defaultOpenCodeHost  = "127.0.0.1"
	defaultOpenCodePort  = 4096
	defaultT3CodeHost    = "127.0.0.1"
	defaultT3CodePort    = 3773
	defaultT3CodeDir     = "/home/thomas/Overlord/core/tools/code"
	defaultT3CodeBaseDir = "~/.local/share/t3code"

	defaultFrontEndPort = 3001
	defaultFrontEndDir  = "~/Overlord/sandbox/phone-code/web"
)

func openCodeHost(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "opencode")
	return stringSetting(settings, openCodeHostKey, defaultOpenCodeHost)
}

func openCodePort(cfg *StackConfig) int {
	settings := serviceSettings(cfg, "opencode")
	return intSetting(settings, openCodePortKey, defaultOpenCodePort)
}

func t3CodeHost(cfg *StackConfig) string {
	settings := serviceSettings(cfg, t3CodeServiceName)
	return stringSetting(settings, t3CodeHostKey, defaultT3CodeHost)
}

func t3CodePort(cfg *StackConfig) int {
	settings := serviceSettings(cfg, t3CodeServiceName)
	return intSetting(settings, t3CodePortKey, defaultT3CodePort)
}

func t3CodeWorkDir(cfg *StackConfig) string {
	settings := serviceSettings(cfg, t3CodeServiceName)
	return stringSetting(settings, t3CodeWorkDirKey, defaultT3CodeDir)
}

func t3CodeBaseDir(cfg *StackConfig) string {
	settings := serviceSettings(cfg, t3CodeServiceName)
	return stringSetting(settings, t3CodeBaseDirKey, defaultT3CodeBaseDir)
}

func t3CodeCommand(cfg *StackConfig) string {
	settings := serviceSettings(cfg, t3CodeServiceName)
	return settings[t3CodeCommandKey]
}

func frontEndPort(cfg *StackConfig) int {
	settings := serviceSettings(cfg, "frontend")
	return intSetting(settings, frontEndPortKey, defaultFrontEndPort)
}

func frontEndWorkDir(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "frontend")
	return stringSetting(settings, frontEndWorkDirKey, defaultFrontEndDir)
}

func tailscaleTarget(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "tailscale")
	return stringSetting(settings, tailscaleTargetKey, T3CodeURL(cfg))
}

func serviceSettings(cfg *StackConfig, name string) map[string]string {
	if cfg == nil || cfg.Services == nil {
		return map[string]string{}
	}
	svcCfg, ok := cfg.Services[name]
	if !ok || svcCfg.Settings == nil {
		return map[string]string{}
	}
	return svcCfg.Settings
}

func intSetting(settings map[string]string, key string, fallback int) int {
	value := settings[key]
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func stringSetting(settings map[string]string, key string, fallback string) string {
	value := settings[key]
	if value == "" {
		return fallback
	}
	return value
}
