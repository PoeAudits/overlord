// Package t3 edits T3 Code's settings.
package t3

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
)

// SetupScriptID is the id of the defaultProjectScripts entry that runs
// `overlord setup` when T3 creates a worktree.
const SetupScriptID = "overlord-setup"

// setupScript is the entry Sync guarantees. async is false so Setup finishes
// before the agent starts.
var setupScript = map[string]any{
	"id":                  SetupScriptID,
	"name":                "Overlord setup",
	"command":             "overlord setup",
	"icon":                "configure",
	"runOnWorktreeCreate": true,
	"async":               false,
}

// setupScriptJSON fixes the key order of the written entry.
const setupScriptJSON = `{"id":"overlord-setup","name":"Overlord setup","command":"overlord setup","icon":"configure","runOnWorktreeCreate":true,"async":false}`

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
		current = []byte("{}\n")
	} else if err != nil {
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
	value, err := findTopLevelValue(settings, scriptsKey)
	if err != nil {
		return nil, "", err
	}
	if value.found {
		scripts, status, err := scriptsWithSetup(settings[value.start:value.end])
		if err != nil {
			return nil, "", err
		}
		return splice(settings, value.start, value.end, scripts), status, nil
	}
	scripts, _, err := scriptsWithSetup([]byte("[]"))
	if err != nil {
		return nil, "", err
	}
	member := []byte(`"` + scriptsKey + `": `)
	member = append(member, scripts...)
	if value.lastMemberEnd < 0 {
		// Empty object: put the member on its own line before the brace.
		insert := append(append([]byte("\n  "), member...), '\n')
		return splice(settings, value.closingBrace, value.closingBrace, insert), Missing, nil
	}
	insert := append([]byte(",\n  "), member...)
	return splice(settings, value.lastMemberEnd, value.lastMemberEnd, insert), Missing, nil
}

// scriptsWithSetup returns the scripts array with the setup script replacing
// the first entry carrying its id (dropping any later ones) or appended, and
// indented as a top-level value of a two-space indented file.
func scriptsWithSetup(array []byte) ([]byte, Status, error) {
	var scripts []json.RawMessage
	if err := json.Unmarshal(array, &scripts); err != nil {
		return nil, "", fmt.Errorf("%s is not an array: %w", scriptsKey, err)
	}
	status := Missing
	kept := make([]json.RawMessage, 0, len(scripts)+1)
	for _, script := range scripts {
		var fields map[string]any
		if json.Unmarshal(script, &fields) != nil || fields["id"] != SetupScriptID {
			kept = append(kept, script)
			continue
		}
		if status != Missing {
			status = Stale
			continue
		}
		status = Stale
		if reflect.DeepEqual(fields, setupScript) {
			status = OK
		}
		kept = append(kept, json.RawMessage(setupScriptJSON))
	}
	if status == Missing {
		kept = append(kept, json.RawMessage(setupScriptJSON))
	}
	compact, err := json.Marshal(kept)
	if err != nil {
		return nil, "", err
	}
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact, "  ", "  "); err != nil {
		return nil, "", err
	}
	return indented.Bytes(), status, nil
}

// topLevelValue locates a member of the settings object by byte offsets.
type topLevelValue struct {
	found      bool
	start, end int
	// lastMemberEnd is the end of the last member's value, or -1 when the
	// object is empty.
	lastMemberEnd int
	closingBrace  int
}

func findTopLevelValue(settings []byte, key string) (topLevelValue, error) {
	decoder := json.NewDecoder(bytes.NewReader(settings))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return topLevelValue{}, errors.New("settings are not a JSON object")
	}
	result := topLevelValue{lastMemberEnd: -1}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return topLevelValue{}, err
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return topLevelValue{}, err
		}
		end := int(decoder.InputOffset())
		if name == key && !result.found {
			result = topLevelValue{found: true, start: end - len(raw), end: end}
		}
		result.lastMemberEnd = end
	}
	if _, err := decoder.Token(); err != nil {
		return topLevelValue{}, err
	}
	result.closingBrace = int(decoder.InputOffset()) - 1
	if _, err := decoder.Token(); err != io.EOF {
		return topLevelValue{}, errors.New("unexpected content after the settings object")
	}
	return result, nil
}

func splice(data []byte, start, end int, insert []byte) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(insert))
	out = append(out, data[:start]...)
	out = append(out, insert...)
	return append(out, data[end:]...)
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
