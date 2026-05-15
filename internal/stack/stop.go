package stack

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/tmux"
)

// StopResult captures stop operation outcomes.
type StopResult struct {
	SessionStopped bool
	TailscaleReset bool
	Warnings       []string
}

// StopStack tears down the StartServer tmux session and clears Tailscale serve
// configuration when the tailscale binary is available.
func StopStack(_ *StackConfig, opts ...tmux.Option) (*StopResult, error) {
	result := &StopResult{}

	session := tmux.NewSession(SessionName, "", opts...)
	exists, err := session.Exists()
	if err != nil {
		return nil, fmt.Errorf("checking session existence: %w", err)
	}
	if exists {
		if err := session.Kill(); err != nil {
			return nil, fmt.Errorf("killing session %s: %w", SessionName, err)
		}
		result.SessionStopped = true
	}

	if _, err := commandLookPath("tailscale"); err != nil {
		result.Warnings = append(result.Warnings, "tailscale not found in PATH; could not reset serve config")
		return result, nil
	}

	cmd := commandExec("tailscale", "serve", "reset")
	if out, err := cmd.CombinedOutput(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("tailscale serve reset failed: %v (%s)", err, string(out)))
		return result, nil
	}
	result.TailscaleReset = true

	return result, nil
}
