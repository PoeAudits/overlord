package sync_test

import (
	"context"
	"fmt"

	"github.com/PoeAudits/overlord/internal/sync"
)

func ExampleRsync_Push() {
	// Create a new rsync instance
	r := sync.NewRsync()

	// Configure push options
	opts := sync.RsyncOptions{
		Source: "/home/user/project",
		Dest:   sync.FormatRemotePath("backup-server", "/backups/project"),
		Excludes: []string{
			"node_modules",
			".git",
			"*.log",
		},
		Delete: true, // Remove files from dest that don't exist in source
	}

	// Perform the sync
	result, err := r.Push(context.Background(), opts)
	if err != nil {
		fmt.Printf("Sync failed: %v\n", err)
		return
	}

	fmt.Printf("Sync completed with exit code %d\n", result.ExitCode)
}

func ExampleRsync_Pull() {
	// Create a new rsync instance
	r := sync.NewRsync()

	// Configure pull options
	opts := sync.RsyncOptions{
		Source: sync.FormatRemotePath("storage-server", "/data/project"),
		Dest:   "/home/user/project",
		Excludes: []string{
			".cache",
			"tmp/",
		},
	}

	// Perform the sync
	result, err := r.Pull(context.Background(), opts)
	if err != nil {
		fmt.Printf("Sync failed: %v\n", err)
		return
	}

	fmt.Printf("Sync completed with exit code %d\n", result.ExitCode)
}

func ExampleRsync_DryRun() {
	// Create a new rsync instance
	r := sync.NewRsync()

	// Configure options for dry run
	opts := sync.RsyncOptions{
		Source: "/home/user/project",
		Dest:   sync.FormatRemotePath("backup-server", "/backups/project"),
		Excludes: []string{
			"node_modules",
		},
		Delete: true,
	}

	// Preview what would be synced
	output, err := r.DryRun(context.Background(), opts)
	if err != nil {
		fmt.Printf("Dry run failed: %v\n", err)
		return
	}

	fmt.Printf("Would sync:\n%s\n", output)
}

func ExampleFormatRemotePath() {
	// Format a remote path for rsync
	remotePath := sync.FormatRemotePath("myserver", "/home/user/data")
	fmt.Println(remotePath)
	// Output: myserver:/home/user/data
}
