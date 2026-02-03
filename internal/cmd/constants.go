package cmd

const (
	// maxProjectNameLen is the maximum allowed length for a project name.
	maxProjectNameLen = 64

	// maxDescDisplayLen is the maximum description length shown in tables and pickers.
	maxDescDisplayLen = 40

	// maxPickerVisibleItems is the default number of visible items in the TUI picker.
	maxPickerVisibleItems = 15

	// minPickerVisibleItems is the minimum number of visible items in the TUI picker.
	minPickerVisibleItems = 5

	// pickerHeaderFooterLines is the number of lines reserved for header/footer in the picker.
	pickerHeaderFooterLines = 5

	// defaultWorktreeBaseDir is the base directory for all worktrees.
	defaultWorktreeBaseDir = "~/Overlord/worktrees"
)

// worktreeAdjectives are used for auto-generating branch names.
var worktreeAdjectives = []string{
	"swift", "bright", "clever", "smooth", "quick",
	"clean", "sharp", "neat", "cool", "fast",
	"bold", "calm", "clear", "crisp", "fresh",
	"light", "prime", "ready", "smart", "steady",
}

// worktreeNouns are used for auto-generating branch names.
var worktreeNouns = []string{
	"fix", "task", "work", "dev", "patch",
	"branch", "code", "build", "test", "run",
	"flow", "sync", "push", "pull", "draft",
	"spike", "probe", "check", "scan", "forge",
}
