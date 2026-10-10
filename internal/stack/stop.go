package stack

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/tmux"
)

// StopResult captures stop operation outcomes.
type StopResult struct {
	SessionStopped bool
	Warnings       []string
}

// StopStack tears down the StartServer tmux session. Tailscale Serve routes are
// intentionally left in place because they may belong to unrelated workflows.
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

	return result, nil
}
