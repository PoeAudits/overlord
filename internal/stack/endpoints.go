package stack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

// OpenCodeURL returns the canonical local URL for the opencode service.
func OpenCodeURL(cfg *StackConfig) string {
	return fmt.Sprintf("http://%s:%d", openCodeHost(cfg), openCodePort(cfg))
}

// T3CodeURL returns the canonical local URL for the t3code service.
func T3CodeURL(cfg *StackConfig) string {
	return fmt.Sprintf("http://%s:%d", t3CodeHost(cfg), t3CodePort(cfg))
}

// RemoteT3CodeURL resolves the current Tailscale HTTPS URL that proxies to
// the local t3code endpoint.
func RemoteT3CodeURL(cfg *StackConfig) (string, error) {
	return remoteURLForTarget(T3CodeURL(cfg))
}

// RemoteOpenCodeURL resolves the current Tailscale HTTPS URL that proxies to
// the local opencode endpoint.
func RemoteOpenCodeURL(cfg *StackConfig) (string, error) {
	return remoteURLForTarget(OpenCodeURL(cfg))
}

func remoteURLForTarget(localURL string) (string, error) {
	target := normalizeURL(localURL)

	cmd := commandExec("tailscale", "serve", "status", "--json")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tailscale serve status: %w", err)
	}

	var status tailscaleServeStatus
	if err := json.Unmarshal(out, &status); err != nil {
		return "", fmt.Errorf("parse tailscale serve status: %w", err)
	}

	for hostPort, web := range status.Web {
		handler, ok := web.Handlers["/"]
		if !ok {
			continue
		}
		if normalizeURL(handler.Proxy) != target {
			continue
		}
		host := strings.TrimSuffix(hostPort, ":443")
		return "https://" + host, nil
	}

	return "", fmt.Errorf("no tailscale web route found for %s", target)
}

type tailscaleServeStatus struct {
	Web map[string]tailscaleServeWeb `json:"Web"`
}

type tailscaleServeWeb struct {
	Handlers map[string]tailscaleServeHandler `json:"Handlers"`
}

type tailscaleServeHandler struct {
	Proxy string `json:"Proxy"`
}

func normalizeURL(input string) string {
	parsed, err := url.Parse(input)
	if err != nil {
		return strings.TrimSuffix(input, "/")
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return strings.TrimSuffix(parsed.String(), "/")
}
