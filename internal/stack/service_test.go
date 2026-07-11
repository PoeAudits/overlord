package stack

import (
	"net/http"
	"os/exec"
	"strings"
	"testing"
)

// Compile-time interface compliance checks.
var (
	_ Service = (*openCodeService)(nil)
	_ Service = (*openCodeDesktopService)(nil)
	_ Service = (*frontEndService)(nil)
	_ Service = (*tailscaleService)(nil)
	_ Service = (*speechOutboxService)(nil)
	_ Service = (*ollamaService)(nil)
	_ Service = (*phoenixService)(nil)
)

func TestDefaultServices_AllEnabled(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode":              {Enabled: true, Settings: map[string]string{}},
			"speech-outbox":         {Enabled: true, Settings: map[string]string{}},
			"opencode-desktop":      {Enabled: true, Settings: map[string]string{}},
			"frontend":              {Enabled: true, Settings: map[string]string{}},
			"tailscale":             {Enabled: true, Settings: map[string]string{}},
			"phoenix":               {Enabled: true, Settings: map[string]string{}},
			"ollama":                {Enabled: true, Settings: map[string]string{"model": "llama3"}},
		},
	}

	services := DefaultServices(cfg)
	if len(services) != 7 {
		t.Fatalf("expected 7 services, got %d", len(services))
	}

	expectedOrder := []string{"opencode", "speech-outbox", "opencode-desktop", "frontend", "tailscale", "phoenix", "ollama"}
	for i, name := range expectedOrder {
		if services[i].Name() != name {
			t.Errorf("service[%d]: expected %q, got %q", i, name, services[i].Name())
		}
	}
	// Verify ollama has a pane command for the model runner.
	if services[6].PaneCommand() == "" {
		t.Error("ollama service should have a non-empty PaneCommand")
	}
}

func TestDefaultServices_FilterDisabled(t *testing.T) {
	cfg := &StackConfig{
		Version: 1,
		Services: map[string]ServiceConfig{
			"opencode":         {Enabled: true, Settings: map[string]string{}},
			"opencode-desktop": {Enabled: false, Settings: map[string]string{}},
			"frontend":         {Enabled: false, Settings: map[string]string{}},
			"tailscale":        {Enabled: true, Settings: map[string]string{}},
			"ollama":           {Enabled: false, Settings: map[string]string{}},
		},
	}

	services := DefaultServices(cfg)
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}

	if services[0].Name() != "opencode" {
		t.Errorf("service[0]: expected %q, got %q", "opencode", services[0].Name())
	}
	if services[1].Name() != "tailscale" {
		t.Errorf("service[1]: expected %q, got %q", "tailscale", services[1].Name())
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
			"opencode":              {Enabled: true},
			"speech-outbox":         {Enabled: true},
			"opencode-desktop":      {Enabled: true},
			"frontend":              {Enabled: true},
			"tailscale":             {Enabled: true},
			"phoenix":               {Enabled: true},
			"ollama":                {Enabled: true},
		},
	}

	// Should not panic with nil settings.
	services := DefaultServices(cfg)
	if len(services) != 7 {
		t.Fatalf("expected 7 services, got %d", len(services))
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
			name:       "speech-outbox",
			svc:        &speechOutboxService{},
			wantName:   "speech-outbox",
			wantWindow: "AliceSpeech",
			wantCmd:    "alice speech-outbox serve --host 127.0.0.1 --port 8090",
			wantEnvLen: 1,
		},
		{
			name:       "opencode-desktop",
			svc:        newOpenCodeDesktopService(DefaultConfig()),
			wantName:   "opencode-desktop",
			wantWindow: "OpenCodeDesktop",
			wantCmd:    "opencode-desktop",
			wantEnvLen: 0,
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
			wantCmd:    "tailscale serve --bg http://127.0.0.1:4096 && tailscale serve --bg --set-path=/speech-outbox http://127.0.0.1:8090",
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
		{
			name:       "phoenix",
			svc:        newPhoenixService(DefaultConfig()),
			wantName:   "phoenix",
			wantWindow: "Phoenix",
			wantCmd:    "uv tool run --from arize-phoenix phoenix serve",
			wantEnvLen: 3,
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
			} else if tt.name == "tailscale" {
				if tt.svc.Command() != tt.wantCmd {
					t.Errorf("Command() = %q, want %q", tt.svc.Command(), tt.wantCmd)
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

func TestPhoenixService_DefaultSettings(t *testing.T) {
	svc := newPhoenixService(DefaultConfig())

	if svc.Name() != "phoenix" {
		t.Fatalf("Name() = %q, want phoenix", svc.Name())
	}
	if svc.WindowName() != "Phoenix" {
		t.Fatalf("WindowName() = %q, want Phoenix", svc.WindowName())
	}
	if svc.Command() != "uv tool run --from arize-phoenix phoenix serve" {
		t.Fatalf("Command() = %q", svc.Command())
	}
	if svc.WorkDir() != "" {
		t.Fatalf("WorkDir() = %q, want empty", svc.WorkDir())
	}

	wantEnv := []string{
		"PHOENIX_HOST=127.0.0.1",
		"PHOENIX_PORT=6006",
		"PHOENIX_WORKING_DIR=/tmp/phoenix-overlord",
	}
	gotEnv := svc.Env()
	if strings.Join(gotEnv, "\n") != strings.Join(wantEnv, "\n") {
		t.Fatalf("Env() = %#v, want %#v", gotEnv, wantEnv)
	}
}

func TestPhoenixService_OverriddenSettings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services["phoenix"] = ServiceConfig{Enabled: true, Settings: map[string]string{
		"host":       "0.0.0.0",
		"port":       "7007",
		"command":    "phoenix serve --dev",
		"workdir":    "/srv/phoenix",
		"data_dir":   "/var/lib/phoenix",
		"health_url": "http://127.0.0.1:7007/healthz",
	}}

	svc := newPhoenixService(cfg)
	if svc.Command() != "phoenix serve --dev" {
		t.Fatalf("Command() = %q", svc.Command())
	}
	if svc.WorkDir() != "/srv/phoenix" {
		t.Fatalf("WorkDir() = %q", svc.WorkDir())
	}

	wantEnv := []string{
		"PHOENIX_HOST=0.0.0.0",
		"PHOENIX_PORT=7007",
		"PHOENIX_WORKING_DIR=/var/lib/phoenix",
	}
	gotEnv := svc.Env()
	if strings.Join(gotEnv, "\n") != strings.Join(wantEnv, "\n") {
		t.Fatalf("Env() = %#v, want %#v", gotEnv, wantEnv)
	}
}

func TestPhoenixService_HealthCheckUsesConfiguredURL(t *testing.T) {
	server := newIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/phoenix-health" {
			t.Fatalf("request path = %q, want /phoenix-health", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))

	cfg := DefaultConfig()
	cfg.Services["phoenix"] = ServiceConfig{Enabled: true, Settings: map[string]string{
		"health_url": server.URL + "/phoenix-health",
	}}
	svc := newPhoenixService(cfg)

	if err := svc.HealthCheck(); err != nil {
		t.Fatalf("HealthCheck() returned error: %v", err)
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

func TestOpenCodeDesktopService_Settings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Services["opencode-desktop"] = ServiceConfig{Enabled: true, Settings: map[string]string{
		openCodeDesktopBinaryKey: "/usr/bin/opencode-desktop",
	}}

	svc := newOpenCodeDesktopService(cfg)
	if svc.Command() != "/usr/bin/opencode-desktop" {
		t.Fatalf("Command() = %q", svc.Command())
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

func TestOpenCodeDesktopService_HealthCheck(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

		t.Run("process found", func(t *testing.T) {
			commandExec = func(name string, args ...string) *exec.Cmd {
				if name != "pgrep" {
					t.Fatalf("command = %q, want pgrep", name)
				}
				return commandWithOutput("123 /usr/lib/opencode-desktop-app")
			}

			svc := newOpenCodeDesktopService(DefaultConfig())
			if err := svc.HealthCheck(); err != nil {
				t.Fatalf("HealthCheck() returned error: %v", err)
		}
	})

	t.Run("process missing", func(t *testing.T) {
		commandExec = func(name string, args ...string) *exec.Cmd {
			return commandWithOutputAndExit("", 1)
		}

		svc := newOpenCodeDesktopService(DefaultConfig())
		err := svc.HealthCheck()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "opencode desktop process not found") {
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
