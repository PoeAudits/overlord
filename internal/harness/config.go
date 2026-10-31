package harness

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/PoeAudits/overlord/internal/jsonedit"
)

// SetClaudeModel returns Claude Code's settings.json content with the top-level
// "model" set to model. Every other byte is kept: an existing value is
// replaced in place and a missing key is inserted as the first member. Empty
// content stands for an absent file.
func SetClaudeModel(settings []byte, model string) ([]byte, error) {
	doc, err := jsonedit.Document(settings)
	if err != nil {
		return nil, err
	}
	object, err := jsonedit.Object(doc)
	if err != nil {
		return nil, err
	}
	value, found, err := jsonedit.Member(doc, object, "model")
	if err != nil {
		return nil, err
	}
	if !found {
		return jsonedit.InsertMember(doc, object, "model", model), nil
	}
	var current string
	if json.Unmarshal(doc[value.Start:value.End], &current) == nil && current == model {
		return doc, nil
	}
	return jsonedit.Splice(doc, value, jsonedit.Marshal(model)), nil
}

// codexModelLine matches a top-level `model = ...` line; group 1 is
// everything up to the value.
var codexModelLine = regexp.MustCompile(`^(\s*model\s*=\s*)`)

// codexTableHeader matches a `[table]` or `[[array]]` header line, after which
// keys are no longer top level.
var codexTableHeader = regexp.MustCompile(`^\s*\[`)

// SetCodexModel returns Codex's config.toml content with the top-level model
// set to model. It edits the one line holding that key, keeping any trailing
// comment, and leaves every other line as it was. A missing key is added as
// the first line. Empty content stands for an absent file.
func SetCodexModel(config []byte, model string) []byte {
	want := tomlString(model)
	lines := strings.SplitAfter(string(config), "\n")
	for i, line := range lines {
		if codexTableHeader.MatchString(line) {
			break
		}
		prefix := codexModelLine.FindString(line)
		if prefix == "" {
			continue
		}
		token, rest := tomlStringToken(line[len(prefix):])
		if token == want || token == "'"+model+"'" {
			return config
		}
		lines[i] = prefix + want + rest
		return []byte(strings.Join(lines, ""))
	}
	return append([]byte("model = "+want+"\n"), config...)
}

// tomlStringToken splits a TOML value at the end of a leading basic or
// literal string. When value does not start with one, the whole value up to
// the line ending is the token.
func tomlStringToken(value string) (token, rest string) {
	if strings.HasPrefix(value, `"`) {
		for i := 1; i < len(value); i++ {
			if value[i] == '\\' {
				i++
				continue
			}
			if value[i] == '"' {
				return value[:i+1], value[i+1:]
			}
		}
	}
	if strings.HasPrefix(value, "'") {
		if end := strings.IndexByte(value[1:], '\''); end >= 0 {
			return value[:end+2], value[end+2:]
		}
	}
	end := len(strings.TrimRight(value, "\r\n"))
	return value[:end], value[end:]
}
