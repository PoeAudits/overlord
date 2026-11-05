// Package t3 edits T3 Code's settings.
package t3

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/PoeAudits/overlord/internal/jsonedit"
)

// RetiredSetupScriptID is the id of the defaultProjectScripts entry earlier
// Syncs registered to run `overlord setup` on every new worktree. Setup now
// runs only when the user invokes it (ADR 0007), so Sync removes the entry.
const RetiredSetupScriptID = "overlord-setup"

const scriptsKey = "defaultProjectScripts"

// RemoveSetupScript removes every defaultProjectScripts entry with
// RetiredSetupScriptID from the settings file under home and reports whether
// it found one. With check it only reports.
//
// Only the defaultProjectScripts value is rewritten: every other byte of the
// file is kept. Inside that array, other entries keep their values but are
// re-indented.
func RemoveSetupScript(home string, check bool) (bool, error) {
	path := SettingsPath(home)
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	updated, found, err := withoutSetupScript(current)
	if err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if check || !found {
		return found, nil
	}
	return true, writeAtomically(path, updated)
}

// withoutSetupScript returns settings without the setup script entries, and
// whether there were any.
func withoutSetupScript(settings []byte) ([]byte, bool, error) {
	doc, err := jsonedit.Document(settings)
	if err != nil {
		return nil, false, err
	}
	root, err := jsonedit.Object(doc)
	if err != nil {
		return nil, false, err
	}
	value, found, err := jsonedit.Member(doc, root, scriptsKey)
	if err != nil || !found {
		return nil, false, err
	}
	var scripts []json.RawMessage
	if err := json.Unmarshal(doc[value.Start:value.End], &scripts); err != nil {
		return nil, false, fmt.Errorf("%s is not an array: %w", scriptsKey, err)
	}
	count := len(scripts)
	kept := slices.DeleteFunc(scripts, func(entry json.RawMessage) bool {
		var script struct {
			ID string `json:"id"`
		}
		return json.Unmarshal(entry, &script) == nil && script.ID == RetiredSetupScriptID
	})
	if len(kept) == count {
		return nil, false, nil
	}
	// Indented as a top-level value of a two-space indented file.
	return jsonedit.Splice(doc, value, jsonedit.MarshalIndented(kept, "  ")), true, nil
}

// writeAtomically writes beside path and renames over it, so T3 never reads
// a half-written file. An existing file keeps its permissions.
func writeAtomically(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	staged := path + ".overlord-sync"
	if err := os.WriteFile(staged, data, mode); err != nil {
		return err
	}
	return os.Rename(staged, path)
}
