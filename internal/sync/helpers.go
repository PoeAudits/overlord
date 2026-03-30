package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"errors"
	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
	"github.com/PoeAudits/overlord/internal/syncthing"

	"io/fs"
)

type linePresenceChecker func(content, requiredLine string) bool

func ensureFileContainsLines(filePath string, requiredLines []string, checker linePresenceChecker) error {
	content, err := os.ReadFile(filePath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	text := string(content)
	checker = resolveLinePresenceChecker(checker)
	missing := missingRequiredLines(text, requiredLines, checker)

	if len(missing) == 0 {
		return nil
	}

	updated := appendLines(text, missing)
	return writeTempAndReplace(filePath, updated)
}

func resolveLinePresenceChecker(checker linePresenceChecker) linePresenceChecker {
	if checker != nil {
		return checker
	}

	return func(content, requiredLine string) bool {
		for _, line := range strings.Split(content, "\n") {
			if strings.TrimSpace(line) == requiredLine {
				return true
			}
		}
		return false
	}
}

func missingRequiredLines(text string, requiredLines []string, checker linePresenceChecker) []string {
	missing := make([]string, 0, len(requiredLines))
	for _, requiredLine := range requiredLines {
		if requiredLine == "" {
			continue
		}
		if !checker(text, requiredLine) {
			missing = append(missing, requiredLine)
		}
	}
	return missing
}

func appendLines(existing string, lines []string) string {
	var builder strings.Builder
	if existing != "" {
		builder.WriteString(existing)
		if !strings.HasSuffix(existing, "\n") {
			builder.WriteString("\n")
		}
	}
	for _, line := range lines {
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}

func writeTempAndReplace(filePath, content string) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(filePath), ".stignore-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", filePath, err)
	}

	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write temp file for %s: %w", filePath, err)
	}
	if err := tmpFile.Chmod(0o644); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to set permissions on temp file for %s: %w", filePath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file for %s: %w", filePath, err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		return fmt.Errorf("failed to replace %s: %w", filePath, err)
	}

	cleanupTmp = false
	return nil
}

// BuildFolder constructs a Syncthing folder definition for a project.
func BuildFolder(projectName, projectPath, localDeviceID, peerDeviceID string) syncthing.Folder {
	return syncthing.Folder{
		ID:    projectName,
		Label: projectName,
		Path:  projectPath,
		Type:  "sendreceive",
		Devices: []syncthing.FolderDevice{
			{DeviceID: localDeviceID},
			{DeviceID: peerDeviceID},
		},
		FSWatcherEnabled: true,
		Copiers:          1,
		Hashers:          1,
		CaseSensitiveFS:  true,
	}
}

// EnsureIgnoreFile makes sure the project .stignore includes the global ignore.
func EnsureIgnoreFile(projectPath string) error {
	globalIgnore, err := GlobalIgnorePath()
	if err != nil {
		return err
	}

	rel, err := filepath.Rel(projectPath, globalIgnore)
	if err != nil {
		return fmt.Errorf("failed to build ignore include path: %w", err)
	}

	includeLine := "#include " + filepath.ToSlash(rel)
	ignorePath := filepath.Join(projectPath, ".stignore")

	if err := ensureFileContainsLines(ignorePath, []string{includeLine}, func(content, requiredLine string) bool {
		return strings.Contains(content, requiredLine)
	}); err != nil {
		return fmt.Errorf("failed to update .stignore: %w", err)
	}

	return nil
}

// GlobalIgnorePath returns the Syncthing global ignore file path.
func GlobalIgnorePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "syncthing", ".stglobalignore"), nil
}

// ProjectPathForPeer resolves the path for a project on a peer machine.
func ProjectPathForPeer(project registry.Project, regBaseDir, peerBaseDir string) (string, error) {
	if project.Path == "" {
		return "", fmt.Errorf("project path is empty")
	}
	if filepath.IsAbs(project.Path) {
		return project.Path, nil
	}
	baseDir := regBaseDir
	if peerBaseDir != "" {
		baseDir = peerBaseDir
	}
	if baseDir == "" {
		return "", fmt.Errorf("peer base_dir required for relative project paths")
	}
	if !filepath.IsAbs(baseDir) {
		return "", fmt.Errorf("peer base_dir required for relative project paths")
	}
	return fileutil.ResolveProjectPath(project.Path, baseDir), nil
}

// deviceEnsurer is the subset of *syncthing.Client used by EnsureDevice.
// It is unexported so callers must pass *syncthing.Client; tests may use a fake.
type deviceEnsurer interface {
	EnsureDevice(ctx context.Context, device syncthing.Device) (bool, error)
}

// EnsureDevice registers the peer as a known Syncthing device.
func EnsureDevice(ctx context.Context, client deviceEnsurer, selection PeerSelection) error {
	deviceID := selection.Peer.Syncthing.DeviceID
	if deviceID == "" {
		return fmt.Errorf("peer '%s' is missing syncthing.device_id", selection.Name)
	}
	device := syncthing.Device{
		DeviceID:    deviceID,
		Name:        selection.Name,
		Compression: "metadata",
	}
	if addr := selection.Peer.Syncthing.Address; addr != "" {
		device.Addresses = []string{addr}
	}
	_, err := client.EnsureDevice(ctx, device)
	return err
}
