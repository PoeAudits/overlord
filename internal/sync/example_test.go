package sync_test

import (
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
	result, err := r.Push(opts)
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
		Source: sync.FormatRemotePathWithUser("admin", "storage-server", "/data/project"),
		Dest:   "/home/user/project",
		Excludes: []string{
			".cache",
			"tmp/",
		},
	}

	// Perform the sync
	result, err := r.Pull(opts)
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
	output, err := r.DryRun(opts)
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

func ExampleFormatRemotePathWithUser() {
	// Format a remote path with user for rsync
	remotePath := sync.FormatRemotePathWithUser("admin", "myserver", "/home/admin/data")
	fmt.Println(remotePath)
	// Output: admin@myserver:/home/admin/data
}

func ExampleParseRemotePath() {
	// Parse a remote path
	user, host, path, err := sync.ParseRemotePath("admin@myserver:/home/admin/data")
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}

	fmt.Printf("User: %q, Host: %q, Path: %q\n", user, host, path)
	// Output: User: "admin", Host: "myserver", Path: "/home/admin/data"
}

func ExampleIsRemotePath() {
	// Check if paths are remote
	fmt.Println(sync.IsRemotePath("myserver:/data"))
	fmt.Println(sync.IsRemotePath("/local/path"))
	// Output:
	// true
	// false
}
