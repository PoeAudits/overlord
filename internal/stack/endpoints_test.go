package stack

import (
	"os/exec"
	"strings"
	"testing"
)

func TestOpenCodeURL_Default(t *testing.T) {
	got := OpenCodeURL(DefaultConfig())
	if got != "http://127.0.0.1:4096" {
		t.Fatalf("OpenCodeURL() = %q, want %q", got, "http://127.0.0.1:4096")
	}
}

func TestOpenCodeURL_FromSettings(t *testing.T) {
	cfg := DefaultConfig()
	opencode := cfg.Services["opencode"]
	opencode.Settings[openCodeHostKey] = "0.0.0.0"
	opencode.Settings[openCodePortKey] = "5000"
	cfg.Services["opencode"] = opencode

	got := OpenCodeURL(cfg)
	if got != "http://0.0.0.0:5000" {
		t.Fatalf("OpenCodeURL() = %q, want %q", got, "http://0.0.0.0:5000")
	}
}

func TestT3CodeURL_Default(t *testing.T) {
	got := T3CodeURL(DefaultConfig())
	if got != "http://127.0.0.1:3773" {
		t.Fatalf("T3CodeURL() = %q, want %q", got, "http://127.0.0.1:3773")
	}
}

func TestT3CodeURL_FromSettings(t *testing.T) {
	cfg := DefaultConfig()
	t3code := cfg.Services["t3code"]
	t3code.Settings[t3CodeHostKey] = "0.0.0.0"
	t3code.Settings[t3CodePortKey] = "4773"
	cfg.Services["t3code"] = t3code

	got := T3CodeURL(cfg)
	if got != "http://0.0.0.0:4773" {
		t.Fatalf("T3CodeURL() = %q, want %q", got, "http://0.0.0.0:4773")
	}
}

func TestNormalizeURL(t *testing.T) {
	if got := normalizeURL("http://127.0.0.1:4096/"); got != "http://127.0.0.1:4096" {
		t.Fatalf("normalizeURL() = %q", got)
	}
}

func TestRemoteOpenCodeURL_ReturnsMatchingRoute(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

	commandExec = func(name string, args ...string) *exec.Cmd {
		return commandWithOutput(`{"Web":{"opencode.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:4096/"}}}}}`)
	}

	got, err := RemoteOpenCodeURL(DefaultConfig())
	if err != nil {
		t.Fatalf("RemoteOpenCodeURL() returned error: %v", err)
	}
	if got != "https://opencode.ts.net" {
		t.Fatalf("RemoteOpenCodeURL() = %q, want %q", got, "https://opencode.ts.net")
	}
}

func TestRemoteT3CodeURL_ReturnsMatchingRoute(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

	commandExec = func(name string, args ...string) *exec.Cmd {
		return commandWithOutput(`{"Web":{"t3code.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:3773/"}}}}}`)
	}

	got, err := RemoteT3CodeURL(DefaultConfig())
	if err != nil {
		t.Fatalf("RemoteT3CodeURL() returned error: %v", err)
	}
	if got != "https://t3code.ts.net" {
		t.Fatalf("RemoteT3CodeURL() = %q, want %q", got, "https://t3code.ts.net")
	}
}

func TestRemoteOpenCodeURL_ReturnsErrorWhenNoRouteMatches(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

	commandExec = func(name string, args ...string) *exec.Cmd {
		return commandWithOutput(`{"Web":{"other.ts.net:443":{"Handlers":{"/":{"Proxy":"http://127.0.0.1:9999"}}}}}`)
	}

	_, err := RemoteOpenCodeURL(DefaultConfig())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no tailscale web route found for http://127.0.0.1:4096") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoteOpenCodeURL_ReturnsParseErrorForInvalidStatusJSON(t *testing.T) {
	prevExec := commandExec
	t.Cleanup(func() {
		commandExec = prevExec
	})

	commandExec = func(name string, args ...string) *exec.Cmd {
		return commandWithOutput("not json")
	}

	_, err := RemoteOpenCodeURL(DefaultConfig())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "parse tailscale serve status") {
		t.Fatalf("unexpected error: %v", err)
	}
}
