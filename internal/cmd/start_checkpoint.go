package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/checkpoint"
	"github.com/PoeAudits/overlord/internal/tmux"
)

var (
	startCaptureCheckpoint = tmux.CaptureCheckpoint
	startSaveCheckpoint    = checkpoint.Save
	startCheckpointPath    = checkpoint.DefaultPath
	startRevertCheckpoint  = checkpoint.Revert
)

var startSaveCmd = &cobra.Command{Use: "save", Short: "Save current tmux workspace checkpoint", Args: cobra.NoArgs, RunE: runStartSave}
var startCheckpointsCmd = &cobra.Command{Use: "checkpoints", Short: "List workspace checkpoints", Args: cobra.NoArgs, RunE: runStartCheckpoints}
var startRevertCmd = &cobra.Command{Use: "revert <index>", Short: "Restore an archived checkpoint as current", Args: cobra.ExactArgs(1), RunE: runStartRevert}

func init() { startCmd.AddCommand(startSaveCmd, startCheckpointsCmd, startRevertCmd) }

func runStartSave(cmd *cobra.Command, args []string) error {
	cp, err := startCaptureCheckpoint(nil)
	if err != nil {
		return cmdError("start save", "failed to capture tmux workspace", err.Error())
	}
	if err := startSaveCheckpoint(startCheckpointPath, &checkpoint.Store{Current: cp}); err != nil {
		return cmdError("start save", "failed to save workspace checkpoint", err.Error())
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Saved workspace checkpoint with %d sessions.\n", len(cp.Sessions))
	return nil
}

func runStartCheckpoints(cmd *cobra.Command, args []string) error {
	store, err := startLoadCheckpoints()
	if err != nil {
		return cmdError("start checkpoints", "failed to load workspace checkpoints", err.Error())
	}
	w := cmd.OutOrStdout()
	if store.Current == nil {
		fmt.Fprintln(w, "No workspace checkpoint saved.")
		return nil
	}
	fmt.Fprintf(w, "current  %s  %d sessions, %d windows, %d panes\n", store.Current.SavedAt.Format("2006-01-02 15:04:05 MST"), len(store.Current.Sessions), windows(store.Current), panes(store.Current))
	for i := range store.Archives {
		cp := &store.Archives[i]
		fmt.Fprintf(w, "%d        %s  %d sessions, %d windows, %d panes\n", i+1, cp.SavedAt.Format("2006-01-02 15:04:05 MST"), len(cp.Sessions), windows(cp), panes(cp))
	}
	return nil
}

func runStartRevert(cmd *cobra.Command, args []string) error {
	index, err := strconv.Atoi(args[0])
	if err != nil || index <= 0 {
		return cmdError("start revert", "invalid checkpoint index", args[0])
	}
	if err := startRevertCheckpoint(startCheckpointPath, index-1); err != nil {
		return cmdError("start revert", "failed to revert workspace checkpoint", err.Error())
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Reverted workspace checkpoint to archive %d.\n", index)
	return nil
}

func windows(cp *checkpoint.Checkpoint) int {
	n := 0
	for _, s := range cp.Sessions {
		n += len(s.Windows)
	}
	return n
}
func panes(cp *checkpoint.Checkpoint) int {
	n := 0
	for _, s := range cp.Sessions {
		for _, w := range s.Windows {
			n += len(w.Panes)
		}
	}
	return n
}
