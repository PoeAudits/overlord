package sync

import (
	"fmt"
	"strings"
)

// ConflictType represents the type of conflict detected
type ConflictType string

const (
	// ConflictLocalOnly indicates a file exists only locally
	ConflictLocalOnly ConflictType = "local-only"

	// ConflictRemoteOnly indicates a file exists only remotely
	ConflictRemoteOnly ConflictType = "remote-only"

	// ConflictBothModified indicates a file has been modified on both sides
	ConflictBothModified ConflictType = "both-modified"
)

// IsValid checks if the conflict type is valid
func (c ConflictType) IsValid() bool {
	switch c {
	case ConflictLocalOnly, ConflictRemoteOnly, ConflictBothModified:
		return true
	}
	return false
}

// String returns a human-readable description of the conflict type
func (c ConflictType) String() string {
	switch c {
	case ConflictLocalOnly:
		return "Local only"
	case ConflictRemoteOnly:
		return "Remote only"
	case ConflictBothModified:
		return "Modified on both sides"
	default:
		return "Unknown"
	}
}

// Conflict represents a file conflict between local and remote
type Conflict struct {
	// Path is the relative path of the conflicting file
	Path string

	// Type is the type of conflict
	Type ConflictType
}

// Validate checks if the conflict is valid
func (c *Conflict) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("conflict path is required")
	}
	if !c.Type.IsValid() {
		return fmt.Errorf("invalid conflict type: %s", c.Type)
	}
	return nil
}

// DetectConflicts detects file conflicts between local and remote paths
// It uses rsync dry-run in both directions to identify differences
//
// Algorithm:
// 1. Run rsync dry-run local→remote (what local would push)
// 2. Run rsync dry-run remote→local (what remote would pull)
// 3. Parse outputs to get list of files that would change each direction
// 4. Files in both lists = potential conflict (both modified)
// 5. Files only in push list = local-only changes
// 6. Files only in pull list = remote-only changes
func DetectConflicts(localPath, remoteHost, remotePath string, excludes []string) ([]Conflict, error) {
	if localPath == "" {
		return nil, fmt.Errorf("local path is required")
	}
	if remoteHost == "" {
		return nil, fmt.Errorf("remote host is required")
	}
	if remotePath == "" {
		return nil, fmt.Errorf("remote path is required")
	}

	rsync := NewRsync()

	// Format remote path
	remote := FormatRemotePath(remoteHost, remotePath)

	// Run dry-run local→remote (push)
	pushOutput, err := rsync.DryRun(RsyncOptions{
		Source:   localPath,
		Dest:     remote,
		Excludes: excludes,
		DryRun:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run push dry-run: %w", err)
	}

	// Run dry-run remote→local (pull)
	pullOutput, err := rsync.DryRun(RsyncOptions{
		Source:   remote,
		Dest:     localPath,
		Excludes: excludes,
		DryRun:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run pull dry-run: %w", err)
	}

	// Parse outputs to extract file lists
	pushFiles := parseRsyncOutput(pushOutput)
	pullFiles := parseRsyncOutput(pullOutput)

	// Detect conflicts
	conflicts := detectConflictsFromFileLists(pushFiles, pullFiles)

	return conflicts, nil
}

// parseRsyncOutput parses rsync dry-run output to extract file paths
// Rsync output format (with -v flag):
// - Lines starting with "sending incremental file list" are headers
// - Lines starting with "sent" or "total size" are footers
// - File paths are listed one per line
// - Directories end with /
// - We skip directories and only track files
func parseRsyncOutput(output string) []string {
	var files []string
	lines := strings.Split(output, "\n")

	inFileList := false
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			continue
		}

		// Start of file list
		if strings.HasPrefix(line, "sending incremental file list") {
			inFileList = true
			continue
		}

		// End of file list (footer lines)
		if strings.HasPrefix(line, "sent ") ||
			strings.HasPrefix(line, "total size is ") ||
			strings.HasPrefix(line, "Number of files:") {
			break
		}

		// Skip non-file-list lines
		if !inFileList {
			continue
		}

		// Skip directories (end with /)
		if strings.HasSuffix(line, "/") {
			continue
		}

		// Skip metadata lines (contain arrows or special markers)
		if strings.Contains(line, "->") ||
			strings.HasPrefix(line, "deleting ") ||
			strings.Contains(line, " bytes/sec") {
			continue
		}

		// This should be a file path
		files = append(files, line)
	}

	return files
}

// detectConflictsFromFileLists analyzes file lists from push and pull operations
// to determine conflict types
func detectConflictsFromFileLists(pushFiles, pullFiles []string) []Conflict {
	var conflicts []Conflict

	// Create sets for efficient lookup
	pushSet := make(map[string]bool)
	pullSet := make(map[string]bool)

	for _, file := range pushFiles {
		pushSet[file] = true
	}
	for _, file := range pullFiles {
		pullSet[file] = true
	}

	// Find files in both lists (both modified)
	for file := range pushSet {
		if pullSet[file] {
			conflicts = append(conflicts, Conflict{
				Path: file,
				Type: ConflictBothModified,
			})
		}
	}

	// Find files only in push list (local-only)
	for file := range pushSet {
		if !pullSet[file] {
			conflicts = append(conflicts, Conflict{
				Path: file,
				Type: ConflictLocalOnly,
			})
		}
	}

	// Find files only in pull list (remote-only)
	for file := range pullSet {
		if !pushSet[file] {
			conflicts = append(conflicts, Conflict{
				Path: file,
				Type: ConflictRemoteOnly,
			})
		}
	}

	return conflicts
}

// DetectConflictsWithRsync detects conflicts using a custom Rsync instance
// This is useful for testing with mocked rsync
func DetectConflictsWithRsync(rsync *Rsync, localPath, remoteHost, remotePath string, excludes []string) ([]Conflict, error) {
	if localPath == "" {
		return nil, fmt.Errorf("local path is required")
	}
	if remoteHost == "" {
		return nil, fmt.Errorf("remote host is required")
	}
	if remotePath == "" {
		return nil, fmt.Errorf("remote path is required")
	}

	// Format remote path
	remote := FormatRemotePath(remoteHost, remotePath)

	// Run dry-run local→remote (push)
	pushOutput, err := rsync.DryRun(RsyncOptions{
		Source:   localPath,
		Dest:     remote,
		Excludes: excludes,
		DryRun:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run push dry-run: %w", err)
	}

	// Run dry-run remote→local (pull)
	pullOutput, err := rsync.DryRun(RsyncOptions{
		Source:   remote,
		Dest:     localPath,
		Excludes: excludes,
		DryRun:   true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run pull dry-run: %w", err)
	}

	// Parse outputs to extract file lists
	pushFiles := parseRsyncOutput(pushOutput)
	pullFiles := parseRsyncOutput(pullOutput)

	// Detect conflicts
	conflicts := detectConflictsFromFileLists(pushFiles, pullFiles)

	return conflicts, nil
}
