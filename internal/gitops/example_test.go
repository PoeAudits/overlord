package gitops_test

import (
	"context"
	"log"

	"github.com/PoeAudits/overlord/internal/gitops"
)

func ExampleGitOps_basic() {
	// Create a GitOps instance for the registry directory
	repoPath := "~/.config/overlord"
	git := gitops.New(repoPath)

	// Stage all changes
	if err := git.Add(context.Background()); err != nil {
		log.Fatal(err)
	}

	// Commit with automatic "overlord: " prefix
	if err := git.Commit(context.Background(), "activate my-project"); err != nil {
		log.Fatal(err)
	}

	// Push to remote (no-op if no remote configured)
	if err := git.Push(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func ExampleGitOps_specificFiles() {
	git := gitops.New("~/.config/overlord")

	// Stage specific files
	if err := git.Add(context.Background(), "registry.yaml", "config.yaml"); err != nil {
		log.Fatal(err)
	}

	// Commit
	if err := git.Commit(context.Background(), "update registry and config"); err != nil {
		log.Fatal(err)
	}
}

func ExampleGitOps_sync() {
	git := gitops.New("~/.config/overlord")

	// Pull latest changes before syncing
	if err := git.Pull(context.Background()); err != nil {
		log.Fatal(err)
	}

	// ... perform sync operations ...

	// Commit and push sync results
	if err := git.Add(context.Background()); err != nil {
		log.Fatal(err)
	}
	if err := git.Commit(context.Background(), "sync completed"); err != nil {
		log.Fatal(err)
	}
	if err := git.Push(context.Background()); err != nil {
		log.Fatal(err)
	}
}
