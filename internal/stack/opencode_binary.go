package stack

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveOpenCodeBinary returns the same opencode binary path used when
// starting the managed OpenCode service.
func ResolveOpenCodeBinary() string {
	return resolveOpenCodeBinary()
}

func resolveOpenCodeBinary() string {
	candidates := opencodeCandidatesFromPath(os.Getenv("PATH"))

	for _, candidate := range candidates {
		if isEphemeralNPXPath(candidate) {
			continue
		}
		if isNPXWrapperScript(candidate) {
			continue
		}
		return candidate
	}

	for _, candidate := range candidates {
		if !isEphemeralNPXPath(candidate) {
			return candidate
		}
	}

	return "opencode"
}

func opencodeCandidatesFromPath(pathEnv string) []string {
	parts := filepath.SplitList(pathEnv)
	seen := map[string]struct{}{}
	var candidates []string

	for _, part := range parts {
		if part == "" {
			continue
		}
		full := filepath.Join(part, "opencode")
		if _, err := os.Stat(full); err != nil {
			continue
		}
		if _, ok := seen[full]; ok {
			continue
		}
		seen[full] = struct{}{}
		candidates = append(candidates, full)
	}

	return candidates
}

func isEphemeralNPXPath(path string) bool {
	return strings.Contains(path, "/.npm/_npx/")
}

func isNPXWrapperScript(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, "npx") && strings.Contains(content, "opencode-ai")
}
