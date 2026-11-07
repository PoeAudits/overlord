package project

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/PoeAudits/overlord/internal/templates"
)

// errNoEndMarker means a file holds a block's begin marker but no end marker
// after it, so Setup cannot tell where Overlord's text stops.
var errNoEndMarker = errors.New("begin marker without end marker; left untouched")

// blockStatus compares the block in the file at path with the freshly
// rendered one.
func blockStatus(path string, item templates.Item, data templates.Data) (Status, string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Missing, "", nil
	}
	if err != nil {
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			return Conflict, "not a file; left untouched", nil
		}
		return "", "", err
	}
	start, end, err := findBlock(content, item.Path, item.Block)
	if errors.Is(err, errNoEndMarker) {
		return Conflict, err.Error(), nil
	}
	if start < 0 {
		return Missing, "", nil
	}
	rendered, err := renderBlock(item, data)
	if err != nil {
		return "", "", err
	}
	if !bytes.Equal(content[start:end], rendered) {
		return Stale, "", nil
	}
	return OK, "", nil
}

// repairBlock creates the file holding the block, replaces the stale block
// wholesale, or inserts the missing block after the last of earlier (the
// blocks the manifest lists before it in the same file) that the file holds,
// else at the top. Bytes outside the block are kept as they are.
func repairBlock(path string, item templates.Item, data templates.Data, earlier []string) error {
	rendered, err := renderBlock(item, data)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return createBlockFile(path, item, data, rendered)
	}
	if err != nil {
		return err
	}
	start, end, err := findBlock(content, item.Path, item.Block)
	if err != nil {
		return err
	}
	if start >= 0 {
		return writeSpliced(path, content, start, end, rendered)
	}
	for i := len(earlier) - 1; i >= 0; i-- {
		if _, end, err := findBlock(content, item.Path, earlier[i]); err == nil && end >= 0 {
			return writeSpliced(path, content, end, end, append([]byte("\n"), rendered...))
		}
	}
	return writeSpliced(path, content, 0, 0, append(rendered, '\n'))
}

func createBlockFile(path string, item templates.Item, data templates.Data, rendered []byte) error {
	if item.CreateWith != "" {
		trailer, err := templates.Render(item.CreateWith, data)
		if err != nil {
			return err
		}
		rendered = append(append(rendered, '\n'), trailer...)
	}
	return writeFile(path, rendered)
}

func writeSpliced(path string, content []byte, start, end int, insert []byte) error {
	spliced := append(append(append([]byte{}, content[:start]...), insert...), content[end:]...)
	return os.WriteFile(path, spliced, 0o644)
}

// renderBlock is the block exactly as it appears in a file: begin marker,
// rendered body, end marker, each ending in a newline.
func renderBlock(item templates.Item, data templates.Data) ([]byte, error) {
	body, err := templates.Render(item.Template, data)
	if err != nil {
		return nil, err
	}
	if len(body) > 0 && body[len(body)-1] != '\n' {
		body = append(body, '\n')
	}
	prefix, suffix := commentMarks(item.Path)
	block := fmt.Appendf(nil, "%soverlord:begin %s v%d%s\n", prefix, item.Block, data.Version, suffix)
	block = append(block, body...)
	return fmt.Appendf(block, "%soverlord:end %s%s\n", prefix, item.Block, suffix), nil
}

// commentMarks are what a marker line in the file at path starts and ends
// with: an HTML comment in Markdown, a # comment in any other file.
func commentMarks(path string) (prefix, suffix string) {
	if strings.HasSuffix(path, ".md") {
		return "<!-- ", " -->"
	}
	return "# ", ""
}

// findBlock returns the byte range of the named block, from the start of its
// begin marker line (any version) through the newline ending its end marker
// line, or -1, -1 when the file has no begin marker.
func findBlock(content []byte, path, name string) (start, end int, err error) {
	quoted := regexp.QuoteMeta(name)
	prefix, suffix := commentMarks(path)
	prefix, suffix = regexp.QuoteMeta(prefix), regexp.QuoteMeta(suffix)
	begin := regexp.MustCompile(`(?m)^` + prefix + `overlord:begin ` + quoted + ` v\d+` + suffix + `$`).FindIndex(content)
	if begin == nil {
		return -1, -1, nil
	}
	finish := regexp.MustCompile(`(?m)^` + prefix + `overlord:end ` + quoted + suffix + `(\n|$)`).FindIndex(content[begin[1]:])
	if finish == nil {
		return -1, -1, errNoEndMarker
	}
	return begin[0], begin[1] + finish[1], nil
}
