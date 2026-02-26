package tmux_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestSessionExists(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		responseError error
		wantExists    bool
		wantErr       bool
	}{
		{
			name:          "session present",
			responseError: nil,
			wantExists:    true,
			wantErr:       false,
		},
		{
			name:          "session missing",
			responseError: &fakeExitError{code: 1, msg: "no session"},
			wantExists:    false,
			wantErr:       false,
		},
		{
			name:          "executor failure",
			responseError: errors.New("unable to talk to tmux"),
			wantExists:    false,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fake := &fakeRunner{responses: []fakeResponse{{err: tt.responseError}}}
			session := tmux.NewSession("proj", "/tmp", tmux.WithRunner(fake.Run))

			exists, err := session.Exists()

			if exists != tt.wantExists {
				t.Fatalf("expected exists=%t, got %t", tt.wantExists, exists)
			}
			if tt.wantErr && err == nil {
				t.Fatalf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			wantCmd := execCall{name: "tmux", args: []string{"has-session", "-t", "proj"}}
			if len(fake.calls) != 1 {
				t.Fatalf("expected 1 executor call, got %d", len(fake.calls))
			}
			if !wantCmd.equal(fake.calls[0]) {
				t.Fatalf("unexpected command: %#v", fake.calls[0])
			}
		})
	}
}

func TestSessionCreate(t *testing.T) {
	tests := []struct {
		name      string
		responses []fakeResponse
		wantErr   error
		wantCalls []execCall
	}{
		{
			name: "creates when session missing",
			responses: []fakeResponse{
				{err: &fakeExitError{code: 1, msg: "missing"}},
				{err: nil},
			},
			wantCalls: []execCall{
				{name: "tmux", args: []string{"has-session", "-t", "proj"}},
				{name: "tmux", args: []string{"new-session", "-d", "-s", "proj", "-n", "base", "-c", "/tmp"}},
			},
		},
		{
			name: "fails when session already exists",
			responses: []fakeResponse{
				{err: nil},
			},
			wantErr:   tmux.ErrSessionExists,
			wantCalls: []execCall{{name: "tmux", args: []string{"has-session", "-t", "proj"}}},
		},
		{
			name: "propagates tmux errors",
			responses: []fakeResponse{
				{err: &fakeExitError{code: 1, msg: "missing"}},
				{err: errors.New("tmux: permission denied")},
			},
			wantErr: errors.New("tmux: permission denied"),
			wantCalls: []execCall{
				{name: "tmux", args: []string{"has-session", "-t", "proj"}},
				{name: "tmux", args: []string{"new-session", "-d", "-s", "proj", "-n", "base", "-c", "/tmp"}},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRunner{responses: append([]fakeResponse(nil), tt.responses...)}
			session := tmux.NewSession("proj", "/tmp", tmux.WithRunner(fake.Run))

			err := session.Create()

			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v but got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			}

			if len(fake.calls) != len(tt.wantCalls) {
				t.Fatalf("expected %d calls, got %d", len(tt.wantCalls), len(fake.calls))
			}
			for i, call := range tt.wantCalls {
				if !call.equal(fake.calls[i]) {
					t.Fatalf("call %d mismatch: want %#v, got %#v", i, call, fake.calls[i])
				}
			}
		})
	}
}

func TestSessionSourceLocal(t *testing.T) {
	tmp := t.TempDir()
	session := tmux.NewSession("proj", tmp)

	if err := session.SourceLocal(); err != nil {
		t.Fatalf("expected no error when file missing, got %v", err)
	}

	script := filepath.Join(tmp, ".tmux.local")
	output := filepath.Join(tmp, "output.txt")
	content := strings.Join([]string{
		"#!/usr/bin/env bash",
		"echo \"${TMUX_SESSION}:${TMUX_PROJECT_DIR}\" > " + output,
	}, "\n")
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	if err := session.SourceLocal(); err != nil {
		t.Fatalf("SourceLocal returned error: %v", err)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}
	got := strings.TrimSpace(string(data))
	want := "proj:" + tmp
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestSessionAttachOrSwitch(t *testing.T) {
	tests := []struct {
		name      string
		tmuxEnv   string
		responses []fakeResponse
		wantErr   error
		wantCall  execCall
	}{
		{
			name:      "attach when outside tmux",
			tmuxEnv:   "",
			responses: []fakeResponse{{err: nil}},
			wantCall:  execCall{name: "tmux", args: []string{"attach", "-t", "proj"}},
		},
		{
			name:      "switch when inside tmux",
			tmuxEnv:   "1",
			responses: []fakeResponse{{err: nil}},
			wantCall:  execCall{name: "tmux", args: []string{"switch-client", "-t", "proj"}},
		},
		{
			name:      "missing session returns ErrSessionNotFound",
			responses: []fakeResponse{{err: &fakeExitError{code: 1, msg: "missing"}}},
			wantErr:   tmux.ErrSessionNotFound,
			wantCall:  execCall{name: "tmux", args: []string{"attach", "-t", "proj"}},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeRunner{responses: append([]fakeResponse(nil), tt.responses...)}
			session := tmux.NewSession("proj", "/tmp", tmux.WithRunner(fake.Run))

			if tt.tmuxEnv != "" {
				t.Setenv("TMUX", tt.tmuxEnv)
			} else {
				t.Setenv("TMUX", "")
			}

			err := session.AttachOrSwitch()

			if tt.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v but got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("expected error %v, got %v", tt.wantErr, err)
				}
			}

			if len(fake.calls) != 1 {
				t.Fatalf("expected 1 executor call, got %d", len(fake.calls))
			}
			if !tt.wantCall.equal(fake.calls[0]) {
				t.Fatalf("unexpected command: %#v", fake.calls[0])
			}
		})
	}
}

type execCall struct {
	name string
	args []string
}

func (c execCall) equal(other execCall) bool {
	if c.name != other.name {
		return false
	}
	if len(c.args) != len(other.args) {
		return false
	}
	for i := range c.args {
		if c.args[i] != other.args[i] {
			return false
		}
	}
	return true
}

type fakeRunner struct {
	responses []fakeResponse
	calls     []execCall
}

func (f *fakeRunner) Run(name string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, execCall{name: name, args: append([]string(nil), args...)})
	if len(f.responses) == 0 {
		return nil, nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp.output, resp.err
}

type fakeResponse struct {
	output []byte
	err    error
}

type fakeExitError struct {
	code int
	msg  string
}

func (e *fakeExitError) Error() string { return e.msg }

func (e *fakeExitError) ExitCode() int { return e.code }
