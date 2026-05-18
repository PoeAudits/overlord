package stack

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/registry"
)

const (
	// healthCheckTimeout is the HTTP timeout for service health checks.
	healthCheckTimeout = 2 * time.Second
)

// Service defines the interface for a managed stack service.
// Each service runs in its own tmux window and can be health-checked.
type Service interface {
	// Name returns the service identifier matching keys in StackConfig.Services.
	Name() string
	// WindowName returns the tmux window name for this service.
	WindowName() string
	// Command returns the shell command to run in the tmux window.
	Command() string
	// WorkDir returns the working directory for the command.
	// An empty string means use the default working directory.
	WorkDir() string
	// Env returns environment variables in KEY=VALUE format.
	Env() []string
	// HealthCheck returns nil if the service is healthy, or an error
	// describing the problem.
	HealthCheck() error
	// PaneCommand returns a command to run in a vertical split pane of this
	// service's window. An empty string means no split is needed.
	PaneCommand() string
}

// DefaultServices returns the daily-driver services in startup order
// (OpenCode, OpenCode Desktop, FrontEnd, Tailscale, Ollama), filtered to only
// those enabled in the provided config. Ollama uses a vertical split pane:
// "ollama serve" in the top pane and the configured model in the bottom pane.
func DefaultServices(cfg *StackConfig) []Service {
	// Define services in startup order.
	builders := []struct {
		name string
		make func(cfg *StackConfig) Service
	}{
		{"opencode", func(cfg *StackConfig) Service { return newOpenCodeService(cfg) }},
		{"opencode-desktop", func(cfg *StackConfig) Service { return newOpenCodeDesktopService(cfg) }},
		{"frontend", func(cfg *StackConfig) Service { return newFrontEndService(cfg) }},
		{"tailscale", func(cfg *StackConfig) Service { return newTailscaleService(cfg) }},
		{"ollama", func(cfg *StackConfig) Service { return newOllamaService(serviceSettings(cfg, "ollama")) }},
	}

	var services []Service
	for _, b := range builders {
		svcCfg, exists := cfg.Services[b.name]
		if !exists || !svcCfg.Enabled {
			continue
		}
		services = append(services, b.make(cfg))
	}
	return services
}

var healthClient = &http.Client{Timeout: healthCheckTimeout}

// httpHealthCheck performs an HTTP GET against the given URL with the
// standard health check timeout. Returns nil on a 2xx response.
func httpHealthCheck(url string) error {
	resp, err := healthClient.Get(url)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// --- OpenCode service ---

type openCodeService struct {
	binary string
	host   string
	port   int
}

func newOpenCodeService(cfg *StackConfig) *openCodeService {
	return &openCodeService{binary: ResolveOpenCodeBinary(), host: openCodeHost(cfg), port: openCodePort(cfg)}
}

func (s *openCodeService) Name() string       { return "opencode" }
func (s *openCodeService) WindowName() string { return "OpenCode" }
func (s *openCodeService) Command() string {
	return fmt.Sprintf("%s serve --hostname %s --port %d", s.binary, s.host, s.port)
}
func (s *openCodeService) WorkDir() string     { return "" }
func (s *openCodeService) Env() []string       { return nil }
func (s *openCodeService) PaneCommand() string { return "" }
func (s *openCodeService) HealthCheck() error {
	return httpHealthCheck(fmt.Sprintf("http://%s:%d/global/health", s.host, s.port))
}

// --- OpenCode Desktop service ---

type openCodeDesktopService struct {
	binary string
}

func newOpenCodeDesktopService(cfg *StackConfig) *openCodeDesktopService {
	return &openCodeDesktopService{binary: openCodeDesktopBinary(cfg)}
}

func (s *openCodeDesktopService) Name() string        { return "opencode-desktop" }
func (s *openCodeDesktopService) WindowName() string  { return "OpenCodeDesktop" }
func (s *openCodeDesktopService) Command() string     { return s.binary }
func (s *openCodeDesktopService) WorkDir() string     { return "" }
func (s *openCodeDesktopService) Env() []string       { return nil }
func (s *openCodeDesktopService) PaneCommand() string { return "" }
func (s *openCodeDesktopService) HealthCheck() error {
	cmd := commandExec("pgrep", "-f", "opencode-desktop|/usr/lib/opencode-desktop")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("opencode desktop process not found: %w", err)
	}
	return nil
}

// --- FrontEnd service ---

type frontEndService struct {
	workDir     string
	port        int
	openCodeAPI string
}

func newFrontEndService(cfg *StackConfig) *frontEndService {
	openCodeURL := OpenCodeURL(cfg)
	return &frontEndService{
		workDir:     expandPathOrOriginal(frontEndWorkDir(cfg)),
		port:        frontEndPort(cfg),
		openCodeAPI: openCodeURL,
	}
}

func expandPathOrOriginal(path string) string {
	expanded, err := registry.ExpandPath(path)
	if err != nil {
		return path
	}
	return expanded
}

func (s *frontEndService) Name() string        { return "frontend" }
func (s *frontEndService) WindowName() string  { return "FrontEnd" }
func (s *frontEndService) Command() string     { return "bun serve.ts" }
func (s *frontEndService) WorkDir() string     { return s.workDir }
func (s *frontEndService) PaneCommand() string { return "" }
func (s *frontEndService) Env() []string {
	return []string{
		fmt.Sprintf("PORT=%d", s.port),
		"OPENCODE_API_URL=" + s.openCodeAPI,
	}
}
func (s *frontEndService) HealthCheck() error {
	return httpHealthCheck(fmt.Sprintf("http://127.0.0.1:%d", s.port))
}

// --- Tailscale service ---

type tailscaleService struct {
	target string
}

func newTailscaleService(cfg *StackConfig) *tailscaleService {
	return &tailscaleService{target: tailscaleTarget(cfg)}
}

func (s *tailscaleService) Name() string        { return "tailscale" }
func (s *tailscaleService) WindowName() string  { return "Tailscale" }
func (s *tailscaleService) Command() string     { return "tailscale serve --bg " + s.target }
func (s *tailscaleService) WorkDir() string     { return "" }
func (s *tailscaleService) Env() []string       { return nil }
func (s *tailscaleService) PaneCommand() string { return "" }
func (s *tailscaleService) HealthCheck() error {
	cmd := commandExec("tailscale", "serve", "status")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("tailscale serve status failed: %w", err)
	}
	if !strings.Contains(string(out), s.serveDisplayTarget()) {
		return fmt.Errorf("tailscale serve missing target %s", s.serveDisplayTarget())
	}
	return nil
}

func (s *tailscaleService) serveDisplayTarget() string {
	parsed, err := url.Parse(s.target)
	if err != nil {
		return s.target
	}
	return parsed.String()
}

// --- Ollama service ---

const defaultOllamaModel = "qwen3.5:2b"
const ollamaModelKey = "model"

type ollamaService struct {
	settings map[string]string
}

func newOllamaService(settings map[string]string) *ollamaService {
	return &ollamaService{settings: settings}
}

func (s *ollamaService) Name() string       { return "ollama" }
func (s *ollamaService) WindowName() string { return "Ollama" }
func (s *ollamaService) Command() string    { return "ollama serve" }
func (s *ollamaService) WorkDir() string    { return "" }
func (s *ollamaService) Env() []string      { return nil }
func (s *ollamaService) HealthCheck() error { return httpHealthCheck("http://127.0.0.1:11434") }
func (s *ollamaService) PaneCommand() string {
	model := s.settings[ollamaModelKey]
	if model == "" {
		model = defaultOllamaModel
	}
	return fmt.Sprintf(
		`bash -c 'until curl -sf http://127.0.0.1:11434 >/dev/null 2>&1; do sleep 0.5; done; ollama run %s'`,
		model,
	)
}
