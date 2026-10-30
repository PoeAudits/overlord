package machine

import (
	"path/filepath"

	"github.com/PoeAudits/overlord/internal/harness"
)

// syncHooks makes every Harness run Sync at session start, so a catalog edit
// is live in the next thread.
func syncHooks(run syncRun) ([]Result, error) {
	var results []Result
	for _, path := range []string{
		filepath.Join(run.sys.Home, ".claude", "settings.json"),
		filepath.Join(run.sys.Home, ".codex", "hooks.json"),
	} {
		result, err := run.editFile(path, "SessionStart hook", harness.EnsureSessionStartHook)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	plugin, err := run.writeGeneratedFile(filepath.Join(run.sys.Home, ".config", "opencode", "plugins", "overlord-sync.js"), harness.OpenCodePlugin, harness.IsGeneratedPlugin, "hand-written plugin")
	if err != nil {
		return nil, err
	}
	return append(results, plugin), nil
}
