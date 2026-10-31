package machine

import (
	"github.com/PoeAudits/overlord/internal/harness"
)

// syncHooks makes every Harness run Sync at session start, so a catalog edit
// is live in the next thread.
func syncHooks(run syncRun) ([]Result, error) {
	var results []Result
	for _, path := range []string{
		harness.ClaudeSettingsPath(run.sys.Home),
		harness.CodexHooksPath(run.sys.Home),
	} {
		result, err := run.editFile(path, "SessionStart hook", harness.EnsureSessionStartHook)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	plugin, err := run.writeGeneratedFile(harness.OpenCodePluginPath(run.sys.Home), harness.OpenCodePlugin, harness.IsGeneratedPlugin, "hand-written plugin")
	if err != nil {
		return nil, err
	}
	return append(results, plugin), nil
}
