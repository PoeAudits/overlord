package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/registry"
)

const (
	ocShimInstallPath = "~/.local/bin/oc"
	ocShimMarker      = "# overlord-managed: oc shim"
)

type ocShimStatus string

const (
	ocShimCurrent  ocShimStatus = "current"
	ocShimMissing  ocShimStatus = "missing"
	ocShimStale    ocShimStatus = "stale"
	ocShimConflict ocShimStatus = "unmanaged"
)

type ocShimState struct {
	Path   string
	Status ocShimStatus
}

type ocShimInstallAction string

const (
	ocShimInstalled      ocShimInstallAction = "installed"
	ocShimUpdated        ocShimInstallAction = "updated"
	ocShimAlreadyCurrent ocShimInstallAction = "already-current"
	ocShimSkipped        ocShimInstallAction = "skipped"
)

type ocShimInstallResult struct {
	Path   string
	Action ocShimInstallAction
	Status ocShimStatus
}

func inspectOCShim() (ocShimState, error) {
	path, err := ocShimPath()
	if err != nil {
		return ocShimState{}, err
	}

	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ocShimState{Path: path, Status: ocShimMissing}, nil
	}
	if err != nil {
		return ocShimState{}, err
	}
	if info.IsDir() {
		return ocShimState{Path: path, Status: ocShimConflict}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ocShimState{}, err
	}

	content := string(data)
	if !strings.Contains(content, ocShimMarker) {
		return ocShimState{Path: path, Status: ocShimConflict}, nil
	}
	if content != ocShimContent() || info.Mode().Perm()&0o111 == 0 {
		return ocShimState{Path: path, Status: ocShimStale}, nil
	}

	return ocShimState{Path: path, Status: ocShimCurrent}, nil
}

func ensureOCShimInstalled() (ocShimInstallResult, error) {
	state, err := inspectOCShim()
	if err != nil {
		return ocShimInstallResult{}, err
	}

	switch state.Status {
	case ocShimCurrent:
		return ocShimInstallResult{Path: state.Path, Action: ocShimAlreadyCurrent, Status: ocShimCurrent}, nil
	case ocShimConflict:
		return ocShimInstallResult{Path: state.Path, Action: ocShimSkipped, Status: ocShimConflict}, nil
	case ocShimMissing:
		if err := writeOCShim(state.Path); err != nil {
			return ocShimInstallResult{}, err
		}
		return ocShimInstallResult{Path: state.Path, Action: ocShimInstalled, Status: ocShimCurrent}, nil
	case ocShimStale:
		if err := writeOCShim(state.Path); err != nil {
			return ocShimInstallResult{}, err
		}
		return ocShimInstallResult{Path: state.Path, Action: ocShimUpdated, Status: ocShimCurrent}, nil
	default:
		return ocShimInstallResult{Path: state.Path, Action: ocShimSkipped, Status: state.Status}, nil
	}
}

func ocShimPath() (string, error) {
	return registry.ExpandPath(ocShimInstallPath)
}

func ocShimContent() string {
	return "#!/bin/sh\n" + ocShimMarker + "\nexec overlord code \"$@\"\n"
}

func writeOCShim(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(path, []byte(ocShimContent()), 0o755)
}
