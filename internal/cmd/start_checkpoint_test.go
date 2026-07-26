package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/checkpoint"
	"github.com/PoeAudits/overlord/internal/tmux"
)

func TestStartSaveStoresCapturedWorkspace(t *testing.T) {
	origCapture, origSave := startCaptureCheckpoint, startSaveCheckpoint
	t.Cleanup(func() { startCaptureCheckpoint, startSaveCheckpoint = origCapture, origSave })
	captured := &checkpoint.Checkpoint{Sessions: []checkpoint.Session{{Name: "work"}}}
	startCaptureCheckpoint = func(tmux.CommandRunner) (*checkpoint.Checkpoint, error) { return captured, nil }
	var saved *checkpoint.Store
	startSaveCheckpoint = func(_ string, store *checkpoint.Store) error { saved = store; return nil }
	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&stderr)
	if err := runStartSave(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.Current != captured {
		t.Fatalf("saved = %#v, want captured checkpoint", saved)
	}
}

func TestStartSaveRejectsOutsideTmux(t *testing.T) {
	origCapture := startCaptureCheckpoint
	t.Cleanup(func() { startCaptureCheckpoint = origCapture })
	startCaptureCheckpoint = func(tmux.CommandRunner) (*checkpoint.Checkpoint, error) { return nil, tmux.ErrNotInSession }
	if err := runStartSave(&cobra.Command{}, nil); err == nil {
		t.Fatal("expected save error outside tmux")
	}
}

func TestStartCheckpointsUsesOneBasedArchiveIndexes(t *testing.T) {
	origLoad := startLoadCheckpoints
	t.Cleanup(func() { startLoadCheckpoints = origLoad })
	startLoadCheckpoints = func() (*checkpoint.Store, error) {
		return &checkpoint.Store{
			Current:  &checkpoint.Checkpoint{SavedAt: time.Unix(2, 0)},
			Archives: []checkpoint.Checkpoint{{SavedAt: time.Unix(1, 0)}},
		}, nil
	}
	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	if err := runStartCheckpoints(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "\n1        ") {
		t.Fatalf("output does not identify the first archive as 1: %q", stdout.String())
	}
}

func TestStartRevertConvertsOneBasedIndex(t *testing.T) {
	origRevert := startRevertCheckpoint
	t.Cleanup(func() { startRevertCheckpoint = origRevert })
	got := -1
	startRevertCheckpoint = func(_ string, index int) error { got = index; return nil }
	cmd := &cobra.Command{}
	cmd.SetErr(&bytes.Buffer{})
	if err := runStartRevert(cmd, []string{"1"}); err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("store index = %d, want 0", got)
	}
}
