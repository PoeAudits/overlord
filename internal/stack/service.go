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

	t3CodeTailscaleServeEnv = "T3CODE_TAILSCALE_SERVE=false"
	t3CodeHealthPath        = "/.well-known/t3/environment"
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
// (OpenCode, T3 Code, FrontEnd, Tailscale, Ollama), filtered to only those
// enabled in the provided config. Ollama uses a vertical split pane:
// "ollama serve" in the top pane and the configured model in the bottom pane.
func DefaultServices(cfg *StackConfig) []Service {
	// Define services in startup order.
	builders := []struct {
		name string
		make func(cfg *StackConfig) Service
	}{
		{"opencode", func(cfg *StackConfig) Service { return newOpenCodeService(cfg) }},
		{t3CodeServiceName, func(cfg *StackConfig) Service { return newT3CodeService(cfg) }},
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

func httpHealthCheckOKOrRedirect(url string) error {
	client := *healthClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
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

// --- T3 Code service ---

type t3CodeService struct {
	host        string
	port        int
	workDir     string
	baseDir     string
	openCodeURL string
	command     string
}

func newT3CodeService(cfg *StackConfig) *t3CodeService {
	return &t3CodeService{
		host:        t3CodeHost(cfg),
		port:        t3CodePort(cfg),
		workDir:     expandPathOrOriginal(t3CodeWorkDir(cfg)),
		baseDir:     t3CodeBaseDir(cfg),
		openCodeURL: OpenCodeURL(cfg),
		command:     t3CodeCommand(cfg),
	}
}

func (s *t3CodeService) Name() string       { return t3CodeServiceName }
func (s *t3CodeService) WindowName() string { return "T3Code" }
func (s *t3CodeService) Command() string {
	if s.command != "" {
		return s.command
	}
	bin := "apps/server/dist/bin.mjs"
	return fmt.Sprintf(
		"test -f %s || { printf '%%s\\n' %s >&2; exit 1; }; %s && node %s serve --host %s --port %d --base-dir %s",
		shellQuote(bin),
		shellQuote("T3 Code server is not built. Run: bun run --cwd apps/web build && bun run --cwd apps/server build"),
		s.seedOpenCodeProviderCommand(),
		shellQuote(bin),
		shellQuote(s.host),
		s.port,
		shellQuote(s.baseDir),
	)
}

func (s *t3CodeService) seedOpenCodeProviderCommand() string {
	script := `const fs=require("fs"),path=require("path");const base=process.argv[1],url=process.argv[2];const file=path.join(base,"userdata","settings.json");fs.mkdirSync(path.dirname(file),{recursive:true});let settings={};try{settings=JSON.parse(fs.readFileSync(file,"utf8"))}catch(e){if(e.code!=="ENOENT")throw e}settings.providers={...settings.providers,opencode:{...settings.providers?.opencode,serverUrl:url}};fs.writeFileSync(file,JSON.stringify(settings,null,2)+"\n")`
	return fmt.Sprintf("node -e %s -- %s %s", shellQuote(script), shellQuote(expandPathOrOriginal(s.baseDir)), shellQuote(s.openCodeURL))
}

func (s *t3CodeService) WorkDir() string     { return s.workDir }
func (s *t3CodeService) Env() []string       { return []string{t3CodeTailscaleServeEnv} }
func (s *t3CodeService) PaneCommand() string { return "" }
func (s *t3CodeService) HealthCheck() error {
	return httpHealthCheckOKOrRedirect(fmt.Sprintf("http://%s:%d%s", s.host, s.port, t3CodeHealthPath))
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

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
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

func (s *tailscaleService) Name() string       { return "tailscale" }
func (s *tailscaleService) WindowName() string { return "Tailscale" }
func (s *tailscaleService) Command() string {
	return fmt.Sprintf(
		"tailscale serve --bg %s; status=$?; if [ $status -eq 0 ]; then for i in 1 2 3 4 5 6 7 8 9 10; do overlord start url --remote >/dev/null 2>&1 && break; sleep 1; done; printf '\\nT3 phone pairing:\\n'; overlord start pair || printf 'Run `overlord start pair` after Tailscale Serve is ready.\\n' >&2; else exit $status; fi; exec ${SHELL:-sh}",
		shellQuote(s.target),
	)
}
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
