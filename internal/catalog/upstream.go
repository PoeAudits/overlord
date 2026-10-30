package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Upstream records where a skill was copied from: a git remote and the commit
// the copy matches.
type Upstream struct {
	Name string
	URL  string
	SHA  string
}

// UpstreamPath returns the skill provenance file in dir.
func UpstreamPath(dir string) string {
	return filepath.Join(dir, "UPSTREAM.md")
}

// LoadUpstream reads the provenance entries of UPSTREAM.md in dir; a missing
// file has none. The file is Markdown kept by hand. An entry is a list item
// of three space-separated fields, the name optionally in backticks:
//
//   - <name> <git URL> <commit SHA, full or abbreviated>
//
// Every other line is prose and ignored.
func LoadUpstream(dir string) ([]Upstream, error) {
	content, err := os.ReadFile(UpstreamPath(dir))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var entries []Upstream
	for _, line := range strings.Split(string(content), "\n") {
		item, ok := strings.CutPrefix(strings.TrimSpace(line), "- ")
		fields := strings.Fields(item)
		if !ok || len(fields) != 3 || !commitSHA.MatchString(fields[2]) {
			continue
		}
		entries = append(entries, Upstream{Name: strings.Trim(fields[0], "`"), URL: fields[1], SHA: fields[2]})
	}
	return entries, nil
}

var commitSHA = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
