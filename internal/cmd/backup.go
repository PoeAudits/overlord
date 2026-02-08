package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/machine"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const defaultBackupRepoPath = "~/.local/share/overlord/backup"

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create and push one-way registry backup",
	Long: `Create a disaster-recovery backup of overlord registry artifacts.

This command copies registry.yaml and peers.yaml into a dedicated local git
repository, commits any changes with a timestamped message, and pushes to
origin/main. It never performs git pull or merge operations.`,
	Args: cobra.NoArgs,
	RunE: runBackup,
}

func init() {
	rootCmd.AddCommand(backupCmd)
}

func runBackup(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	ctx := context.Background()

	repoPath, err := registry.ExpandPath(defaultBackupRepoPath)
	if err != nil {
		return fmt.Errorf("failed to resolve backup repo path: %w", err)
	}
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup repo directory: %w", err)
	}

	if err := ensureBackupRepoInitialized(ctx, repoPath); err != nil {
		return err
	}

	if err := ensureBackupRemoteConfigured(ctx, repoPath); err != nil {
		return err
	}

	registryPath, peersPath, err := sourceArtifactPaths()
	if err != nil {
		return err
	}

	if err := copyFileWithMode(registryPath, filepath.Join(repoPath, "registry.yaml")); err != nil {
		return fmt.Errorf("failed to copy registry.yaml to backup repo: %w", err)
	}
	if err := copyFileWithMode(peersPath, filepath.Join(repoPath, "peers.yaml")); err != nil {
		return fmt.Errorf("failed to copy peers.yaml to backup repo: %w", err)
	}

	if err := runGit(ctx, repoPath, "add", "registry.yaml", "peers.yaml"); err != nil {
		return err
	}

	hasChanges, err := hasStagedChanges(ctx, repoPath)
	if err != nil {
		return err
	}

	if hasChanges {
		if err := runGit(ctx, repoPath, "commit", "-m", backupCommitMessage(time.Now().UTC())); err != nil {
			return err
		}
		fmt.Printf("%s committed registry backup\n", s.Success.Render("✓"))
	} else {
		fmt.Printf("%s no artifact changes to commit\n", s.Info.Render("Info:"))
	}

	if err := runGit(ctx, repoPath, "push", "-u", "origin", "main"); err != nil {
		return err
	}

	fmt.Printf("%s pushed backup to origin/main\n", s.Success.Render("✓"))
	fmt.Printf("%s %s\n", s.Label.Render("Backup repo:"), formatPathWithTilde(repoPath))
	return nil
}

func sourceArtifactPaths() (string, string, error) {
	registryPath, err := registry.ExpandPath(registry.DefaultRegistryPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve registry path: %w", err)
	}
	peersPath, err := registry.ExpandPath(machine.DefaultPeersPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve peers path: %w", err)
	}

	if _, err := os.Stat(registryPath); err != nil {
		return "", "", fmt.Errorf("registry artifact missing: %s", registryPath)
	}
	if _, err := os.Stat(peersPath); err != nil {
		return "", "", fmt.Errorf("peer artifact missing: %s", peersPath)
	}

	return registryPath, peersPath, nil
}

func ensureBackupRepoInitialized(ctx context.Context, repoPath string) error {
	if err := runGit(ctx, repoPath, "rev-parse", "--is-inside-work-tree"); err == nil {
		return nil
	}

	if err := runGit(ctx, repoPath, "init", "-b", "main"); err != nil {
		return err
	}
	return nil
}

func ensureBackupRemoteConfigured(ctx context.Context, repoPath string) error {
	if err := runGit(ctx, repoPath, "remote", "get-url", "origin"); err == nil {
		return nil
	}

	remoteURL, err := promptBackupRemoteURL()
	if err != nil {
		return err
	}
	if remoteURL == "" {
		return fmt.Errorf("backup remote is not configured\nSet origin with: git -C %s remote add origin git@github.com:<user>/<repo>.git", repoPath)
	}

	if err := runGit(ctx, repoPath, "remote", "add", "origin", remoteURL); err != nil {
		return err
	}
	return nil
}

func promptBackupRemoteURL() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", nil
	}

	fmt.Print("Backup remote URL (git@github.com:<user>/<repo>.git): ")
	reader := bufio.NewReader(os.Stdin)
	remoteURL, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read backup remote URL: %w", err)
	}
	return strings.TrimSpace(remoteURL), nil
}

func hasStagedChanges(ctx context.Context, repoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "diff", "--cached", "--quiet", "--", "registry.yaml", "peers.yaml")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}

	return false, fmt.Errorf("failed to check staged changes: %w", err)
}

func runGit(ctx context.Context, repoPath string, args ...string) error {
	command := append([]string{"-C", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "git", command...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w (output: %s)", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func backupCommitMessage(now time.Time) string {
	return fmt.Sprintf("overlord backup: registry + peers at %s", now.Format(time.RFC3339))
}
