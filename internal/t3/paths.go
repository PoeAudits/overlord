package t3

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Every path Overlord uses under T3 Code's data directory, ~/.t3.

// SettingsPath is T3 Code's global settings file under home.
func SettingsPath(home string) string {
	return filepath.Join(userdata(home), "settings.json")
}

// LauncherPath is the t3 command line launcher, which is not on PATH.
func LauncherPath(home string) string {
	return filepath.Join(home, ".t3", "bin", "t3")
}

func stateDatabasePath(home string) string {
	return filepath.Join(userdata(home), "statev2.sqlite")
}

func userdata(home string) string {
	return filepath.Join(home, ".t3", "userdata")
}

// DefaultOrigin is where T3 Code's server listens unless configured otherwise.
const DefaultOrigin = "http://127.0.0.1:3773"

// Origin is the origin T3 Code's server last ran on, read from its
// server-runtime file, or DefaultOrigin when that file is absent or unreadable.
func Origin(home string) string {
	data, err := os.ReadFile(filepath.Join(userdata(home), "server-runtime.json"))
	if err != nil {
		return DefaultOrigin
	}
	var runtime struct {
		Origin string `json:"origin"`
	}
	if json.Unmarshal(data, &runtime) != nil || runtime.Origin == "" {
		return DefaultOrigin
	}
	return runtime.Origin
}
