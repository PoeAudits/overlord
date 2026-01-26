package gitops_test

import (
	"fmt"
	"log"

	"github.com/PoeAudits/overlord/internal/gitops"
)

func ExampleGitOps_basic() {
	// Create a GitOps instance for the registry directory
	repoPath := "~/.config/overlord"
	git := gitops.New(repoPath)

	// Check if there are uncommitted changes
	hasChanges, err := git.HasChanges()
	if err != nil {
		log.Fatal(err)
	}

	if hasChanges {
		fmt.Println("Repository has uncommitted changes")
	}

	// Stage all changes
	if err := git.Add(); err != nil {
		log.Fatal(err)
	}

	// Commit with automatic "overlord: " prefix
	if err := git.Commit("activate my-project"); err != nil {
		log.Fatal(err)
	}

	// Push to remote (no-op if no remote configured)
	if err := git.Push(); err != nil {
		log.Fatal(err)
	}
}

func ExampleGitOps_specificFiles() {
	git := gitops.New("~/.config/overlord")

	// Stage specific files
	if err := git.Add("registry.yaml", "config.yaml"); err != nil {
		log.Fatal(err)
	}

	// Commit
	if err := git.Commit("update registry and config"); err != nil {
		log.Fatal(err)
	}
}

func ExampleGitOps_sync() {
	git := gitops.New("~/.config/overlord")

	// Pull latest changes before syncing
	if err := git.Pull(); err != nil {
		log.Fatal(err)
	}

	// ... perform sync operations ...

	// Check if sync created changes
	hasChanges, err := git.HasChanges()
	if err != nil {
		log.Fatal(err)
	}

	if hasChanges {
		// Commit and push sync results
		if err := git.Add(); err != nil {
			log.Fatal(err)
		}
		if err := git.Commit("sync completed"); err != nil {
			log.Fatal(err)
		}
		if err := git.Push(); err != nil {
			log.Fatal(err)
		}
	}
}
