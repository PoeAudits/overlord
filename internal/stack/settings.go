package stack

import (
	"fmt"
	"strconv"
)

const (
	openCodeHostKey          = "host"
	openCodePortKey          = "port"
	openCodeDesktopBinaryKey = "binary"

	frontEndPortKey    = "port"
	frontEndWorkDirKey = "workdir"

	tailscaleTargetKey = "target"

	phoenixHostKey      = "host"
	phoenixPortKey      = "port"
	phoenixCommandKey   = "command"
	phoenixWorkDirKey   = "workdir"
	phoenixDataDirKey   = "data_dir"
	phoenixHealthURLKey = "health_url"

	defaultOpenCodeHost          = "127.0.0.1"
	defaultOpenCodePort          = 4096
	defaultOpenCodeDesktopBinary = "opencode-desktop"

	defaultFrontEndPort = 3001
	defaultFrontEndDir  = "~/Overlord/sandbox/phone-code/web"

	defaultPhoenixHost    = "127.0.0.1"
	defaultPhoenixPort    = 6006
	defaultPhoenixCommand = "uv tool run --from arize-phoenix phoenix serve"
	defaultPhoenixDataDir = "/tmp/phoenix-overlord"
)

func openCodeHost(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "opencode")
	return stringSetting(settings, openCodeHostKey, defaultOpenCodeHost)
}

func openCodePort(cfg *StackConfig) int {
	settings := serviceSettings(cfg, "opencode")
	return intSetting(settings, openCodePortKey, defaultOpenCodePort)
}

func openCodeDesktopBinary(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "opencode-desktop")
	return stringSetting(settings, openCodeDesktopBinaryKey, defaultOpenCodeDesktopBinary)
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
	return stringSetting(settings, tailscaleTargetKey, OpenCodeURL(cfg))
}

func phoenixHost(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "phoenix")
	return stringSetting(settings, phoenixHostKey, defaultPhoenixHost)
}

func phoenixPort(cfg *StackConfig) int {
	settings := serviceSettings(cfg, "phoenix")
	return intSetting(settings, phoenixPortKey, defaultPhoenixPort)
}

func phoenixCommand(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "phoenix")
	return stringSetting(settings, phoenixCommandKey, defaultPhoenixCommand)
}

func phoenixWorkDir(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "phoenix")
	return stringSetting(settings, phoenixWorkDirKey, "")
}

func phoenixDataDir(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "phoenix")
	return stringSetting(settings, phoenixDataDirKey, defaultPhoenixDataDir)
}

func phoenixHealthURL(cfg *StackConfig) string {
	settings := serviceSettings(cfg, "phoenix")
	if url := settings[phoenixHealthURLKey]; url != "" {
		return url
	}
	return fmt.Sprintf("http://%s:%d/", phoenixHost(cfg), phoenixPort(cfg))
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
