// Package t3 edits T3 Code's settings.
package t3

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/PoeAudits/overlord/internal/jsonedit"
)

// SetupScriptID is the id of the defaultProjectScripts entry that runs
// `overlord setup` when T3 creates a worktree.
const SetupScriptID = "overlord-setup"

// setupScript is the entry Sync guarantees, in the field order it is written.
// Async is false so Setup finishes before the agent starts.
var setupScript = script{
	ID:                  SetupScriptID,
	Name:                "Overlord setup",
	Command:             "overlord setup",
	Icon:                "configure",
	RunOnWorktreeCreate: true,
	Async:               false,
}

type script struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	Command             string `json:"command"`
	Icon                string `json:"icon"`
	RunOnWorktreeCreate bool   `json:"runOnWorktreeCreate"`
	Async               bool   `json:"async"`
}

const scriptsKey = "defaultProjectScripts"

// Status is the state of the setup script in the settings file.
type Status string

const (
	// Missing: no entry with SetupScriptID, or no settings file.
	Missing Status = "missing"
	// Stale: an entry with SetupScriptID whose fields differ, or several.
	Stale Status = "stale"
	OK    Status = "ok"
)

// EnsureSetupScript makes defaultProjectScripts in the settings file under
// home contain exactly one setup script entry with the expected fields, and
// returns the status it found. With check it only reports.
//
// Only the defaultProjectScripts value is rewritten: every other byte of the
// file is kept. Inside that array, other entries keep their values but are
// re-indented.
func EnsureSetupScript(home string, check bool) (Status, error) {
	path := SettingsPath(home)
	current, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		current, err = []byte("{}\n"), nil
	}
	if err != nil {
		return "", err
	}
	updated, status, err := withSetupScript(current)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	if check || status == OK {
		return status, nil
	}
	return status, writeAtomically(path, updated)
}

// withSetupScript returns settings with the setup script ensured.
func withSetupScript(settings []byte) ([]byte, Status, error) {
	doc, err := jsonedit.Document(settings)
	if err != nil {
		return nil, "", err
	}
	root, err := jsonedit.Object(doc)
	if err != nil {
		return nil, "", err
	}
	value, found, err := jsonedit.Member(doc, root, scriptsKey)
	if err != nil {
		return nil, "", err
	}
	if !found {
		return jsonedit.InsertMember(doc, root, scriptsKey, []script{setupScript}), Missing, nil
	}
	scripts, status, err := scriptsWithSetup(doc[value.Start:value.End])
	if err != nil {
		return nil, "", err
	}
	// Indented as a top-level value of a two-space indented file.
	return jsonedit.Splice(doc, value, jsonedit.MarshalIndented(scripts, "  ")), status, nil
}

// scriptsWithSetup returns the scripts with the setup script replacing the
// first entry carrying its id (dropping any later ones) or appended.
func scriptsWithSetup(array []byte) ([]any, Status, error) {
	var scripts []json.RawMessage
	if err := json.Unmarshal(array, &scripts); err != nil {
		return nil, "", fmt.Errorf("%s is not an array: %w", scriptsKey, err)
	}
	want := setupScriptFields()
	status := Missing
	kept := make([]any, 0, len(scripts)+1)
	for _, entry := range scripts {
		var fields map[string]any
		if json.Unmarshal(entry, &fields) != nil || fields["id"] != SetupScriptID {
			kept = append(kept, entry)
			continue
		}
		if status != Missing {
			status = Stale
			continue
		}
		status = Stale
		if reflect.DeepEqual(fields, want) {
			status = OK
		}
		kept = append(kept, setupScript)
	}
	if status == Missing {
		kept = append(kept, setupScript)
	}
	return kept, status, nil
}

// setupScriptFields is setupScript as a decoded JSON object, to compare with
// an entry read from the file.
func setupScriptFields() map[string]any {
	var fields map[string]any
	if err := json.Unmarshal([]byte(jsonedit.Marshal(setupScript)), &fields); err != nil {
		panic(err)
	}
	return fields
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
