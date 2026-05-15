package stack

import (
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Compile-time interface compliance checks.
var (
	_ Service = (*openCodeService)(nil)
	_ Service = (*t3CodeService)(nil)
	_ Service = (*frontEndService)(nil)
	_ Service = (*tailscaleService)(nil)
	_ Service = (*ollamaService)(nil)
)

func TestDefaultServices_AllEnabled(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode":  {Enabled: true, Settings: map[string]string{}},
			"t3code":    {Enabled: true, Settings: map[string]string{}},
			"frontend":  {Enabled: true, Settings: map[string]string{}},
			"tailscale": {Enabled: true, Settings: map[string]string{}},
			"ollama":    {Enabled: true, Settings: map[string]string{"model": "llama3"}},
		},
	}

	services := DefaultServices(cfg)
	if len(services) != 5 {
		t.Fatalf("expected 5 services, got %d", len(services))
	}

	expectedOrder := []string{"opencode", "t3code", "frontend", "tailscale", "ollama"}
	for i, name := range expectedOrder {
		if services[i].Name() != name {
			t.Errorf("service[%d]: expected %q, got %q", i, name, services[i].Name())
		}
	}
	// Verify ollama has a pane command for the model runner.
	if services[4].PaneCommand() == "" {
		t.Error("ollama service should have a non-empty PaneCommand")
	}
}

func TestDefaultServices_FilterDisabled(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode":  {Enabled: true, Settings: map[string]string{}},
			"t3code":    {Enabled: true, Settings: map[string]string{}},
			"frontend":  {Enabled: false, Settings: map[string]string{}},
			"tailscale": {Enabled: true, Settings: map[string]string{}},
			"ollama":    {Enabled: false, Settings: map[string]string{}},
		},
	}

	services := DefaultServices(cfg)
	if len(services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(services))
	}

	if services[0].Name() != "opencode" {
		t.Errorf("service[0]: expected %q, got %q", "opencode", services[0].Name())
	}
	if services[1].Name() != "t3code" {
		t.Errorf("service[1]: expected %q, got %q", "t3code", services[1].Name())
	}
	if services[2].Name() != "tailscale" {
		t.Errorf("service[2]: expected %q, got %q", "tailscale", services[2].Name())
	}
}

func TestDefaultServices_MissingServiceConfig(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode": {Enabled: true, Settings: map[string]string{}},
			// frontend, tailscale, ollama missing from config
		},
	}

	services := DefaultServices(cfg)
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].Name() != "opencode" {
		t.Errorf("expected %q, got %q", "opencode", services[0].Name())
	}
}

func TestDefaultServices_NilSettings(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode":  {Enabled: true},
			"t3code":    {Enabled: true},
			"frontend":  {Enabled: true},
			"tailscale": {Enabled: true},
			"ollama":    {Enabled: true},
		},
	}

	// Should not panic with nil settings.
	services := DefaultServices(cfg)
	if len(services) != 5 {
		t.Fatalf("expected 5 services, got %d", len(services))
	}
}

func TestServiceProperties(t *testing.T) {
	tests := []struct {
		name       string
		svc        Service
		wantName   string
		wantWindow string
		wantCmd    string
		wantEnvLen int
	}{
		{
			name:       "opencode",
			svc:        newOpenCodeService(DefaultConfig()),
			wantName:   "opencode",
			wantWindow: "OpenCode",
			wantCmd:    "serve --hostname 127.0.0.1 --port 4096",
			wantEnvLen: 0,
		},
		{
			name:       "t3code",
			svc:        newT3CodeService(DefaultConfig()),
			wantName:   "t3code",
			wantWindow: "T3Code",
			wantCmd:    "node 'apps/server/dist/bin.mjs' serve --host '127.0.0.1' --port 3773 --base-dir '~/.local/share/t3code'",
			wantEnvLen: 1,
		},
		{
			name:       "frontend",
			svc:        newFrontEndService(DefaultConfig()),
			wantName:   "frontend",
			wantWindow: "FrontEnd",
			wantCmd:    "bun serve.ts",
			wantEnvLen: 2,
		},
		{
			name:       "tailscale",
			svc:        newTailscaleService(DefaultConfig()),
			wantName:   "tailscale",
			wantWindow: "Tailscale",
			wantCmd:    "tailscale serve --bg http://127.0.0.1:3773",
			wantEnvLen: 0,
		},
		{
			name:       "ollama",
			svc:        newOllamaService(map[string]string{"model": "llama3"}),
			wantName:   "ollama",
			wantWindow: "Ollama",
			wantCmd:    "ollama serve",
			wantEnvLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.svc.Name() != tt.wantName {
				t.Errorf("Name() = %q, want %q", tt.svc.Name(), tt.wantName)
			}
			if tt.svc.WindowName() != tt.wantWindow {
				t.Errorf("WindowName() = %q, want %q", tt.svc.WindowName(), tt.wantWindow)
			}
			if tt.name == "opencode" {
				if !strings.HasSuffix(tt.svc.Command(), tt.wantCmd) {
					t.Errorf("Command() = %q, want suffix %q", tt.svc.Command(), tt.wantCmd)
				}
			} else if tt.name == "t3code" {
				if !strings.Contains(tt.svc.Command(), tt.wantCmd) {
					t.Errorf("Command() = %q, want substring %q", tt.svc.Command(), tt.wantCmd)
				}
			} else if tt.svc.Command() != tt.wantCmd {
				t.Errorf("Command() = %q, want %q", tt.svc.Command(), tt.wantCmd)
			}
			env := tt.svc.Env()
			if len(env) != tt.wantEnvLen {
				t.Errorf("Env() length = %d, want %d", len(env), tt.wantEnvLen)
			}
		})
	}
}

func TestOllamaService_PaneCommand(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
		want     string
	}{
		{
			name:     "default model",
			settings: map[string]string{},
			want:     `bash -c 'until curl -sf http://127.0.0.1:11434 >/dev/null 2>&1; do sleep 0.5; done; ollama run qwen3.5:2b'`,
		},
		{
			name:     "custom model",
			settings: map[string]string{ollamaModelKey: "llama3"},
			want:     `bash -c 'until curl -sf http://127.0.0.1:11434 >/dev/null 2>&1; do sleep 0.5; done; ollama run llama3'`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newOllamaService(tt.settings)
			if got := svc.PaneCommand(); got != tt.want {
				t.Errorf("PaneCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFrontEndService_WorkDir(t *testing.T) {
	svc := newFrontEndService(DefaultConfig())
	workDir := svc.WorkDir()
	if workDir == "" {
		t.Error("expected non-empty WorkDir for frontend service")
	}
	// Should be expanded (no ~ prefix).
	if len(workDir) > 0 && workDir[0] == '~' {
		t.Errorf("WorkDir should be expanded, got %q", workDir)
	}
}

func TestT3CodeService_Settings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services["t3code"] = ServiceConfig{Enabled: true, Settings: map[string]string{
		t3CodeHostKey:    "0.0.0.0",
		t3CodePortKey:    "4773",
		t3CodeWorkDirKey: "/tmp/t3code",
		t3CodeBaseDirKey: "/tmp/t3code-state",
	}}

	svc := newT3CodeService(cfg)
	if !strings.Contains(svc.Command(), "node 'apps/server/dist/bin.mjs' serve --host '0.0.0.0' --port 4773 --base-dir '/tmp/t3code-state'") {
		t.Fatalf("Command() = %q", svc.Command())
	}
	if svc.WorkDir() != "/tmp/t3code" {
		t.Fatalf("WorkDir() = %q, want %q", svc.WorkDir(), "/tmp/t3code")
	}
}

func TestT3CodeService_CommandSeedsOpenCodeProvider(t *testing.T) {
	cfg := DefaultConfig()
	svc := newT3CodeService(cfg)
	cmd := svc.Command()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() returned error: %v", err)
	}

	for _, want := range []string{
		"test -f 'apps/server/dist/bin.mjs'",
		"T3 Code server is not built. Run: bun run --cwd apps/web build && bun run --cwd apps/server build",
		"serverUrl:url",
		shellQuote(home + "/.local/share/t3code"),
		"'http://127.0.0.1:4096'",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("Command() missing %q:\n%s", want, cmd)
		}
	}
}

func TestT3CodeService_CommandOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services["t3code"] = ServiceConfig{Enabled: true, Settings: map[string]string{
		t3CodeCommandKey: "bun --cwd apps/server dev",
	}}

	svc := newT3CodeService(cfg)
	if svc.Command() != "bun --cwd apps/server dev" {
		t.Fatalf("Command() = %q", svc.Command())
	}
}

func TestT3CodeService_Env(t *testing.T) {
	svc := newT3CodeService(DefaultConfig())
	env := svc.Env()
	if len(env) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(env))
	}
	if env[0] != "T3CODE_TAILSCALE_SERVE=false" {
		t.Errorf("expected T3CODE_TAILSCALE_SERVE=false, got %q", env[0])
	}
}

func TestFrontEndService_Env(t *testing.T) {
	svc := newFrontEndService(DefaultConfig())
	env := svc.Env()
	if len(env) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(env))
	}
	if env[0] != "PORT=3001" {
		t.Errorf("expected PORT=3001, got %q", env[0])
	}
	if env[1] != "OPENCODE_API_URL=http://127.0.0.1:4096" {
		t.Errorf("expected OPENCODE_API_URL=http://127.0.0.1:4096, got %q", env[1])
	}
}

func TestOpenCodeService_HealthCheck(t *testing.T) {
	t.Run("healthy response passes", func(t *testing.T) {
		server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/global/health" {
				t.Fatalf("request path = %q, want %q", r.URL.Path, "/global/health")
			}
			w.WriteHeader(http.StatusOK)
		}))

		host, port := mustServerHostPort(t, server.URL)
		svc := &openCodeService{host: host, port: port}

		if err := svc.HealthCheck(); err != nil {
			t.Fatalf("HealthCheck() returned error: %v", err)
		}
	})

	t.Run("non-2xx response fails", func(t *testing.T) {
		server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))

		host, port := mustServerHostPort(t, server.URL)
		svc := &openCodeService{host: host, port: port}

		err := svc.HealthCheck()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 503") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestT3CodeService_HealthCheck(t *testing.T) {
	t.Run("healthy redirect passes", func(t *testing.T) {
		server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/.well-known/t3/environment" {
				t.Fatalf("request path = %q, want %q", r.URL.Path, "/.well-known/t3/environment")
			}
			w.WriteHeader(http.StatusFound)
		}))

		host, port := mustServerHostPort(t, server.URL)
		svc := &t3CodeService{host: host, port: port}

		if err := svc.HealthCheck(); err != nil {
			t.Fatalf("HealthCheck() returned error: %v", err)
		}
	})

	t.Run("non-2xx-or-3xx response fails", func(t *testing.T) {
		server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))

		host, port := mustServerHostPort(t, server.URL)
		svc := &t3CodeService{host: host, port: port}

		err := svc.HealthCheck()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "status 404") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestFrontEndService_HealthCheck(t *testing.T) {
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("request path = %q, want root path", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))

	_, port := mustServerHostPort(t, server.URL)
	svc := &frontEndService{port: port}

	if err := svc.HealthCheck(); err != nil {
		t.Fatalf("HealthCheck() returned error: %v", err)
	}
}

func TestTailscaleService_HealthCheck(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

	t.Run("matching target is healthy", func(t *testing.T) {
		commandExec = func(name string, args ...string) *exec.Cmd {
			return commandWithOutput("https://demo.ts.net (Proxy) -> http://127.0.0.1:4096\n")
		}

		svc := &tailscaleService{target: "http://127.0.0.1:4096"}
		if err := svc.HealthCheck(); err != nil {
			t.Fatalf("HealthCheck() returned error: %v", err)
		}
	})

	t.Run("missing target is unhealthy", func(t *testing.T) {
		commandExec = func(name string, args ...string) *exec.Cmd {
			return commandWithOutput("https://demo.ts.net (Proxy) -> http://127.0.0.1:9999\n")
		}

		svc := &tailscaleService{target: "http://127.0.0.1:4096"}
		err := svc.HealthCheck()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "missing target http://127.0.0.1:4096") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
