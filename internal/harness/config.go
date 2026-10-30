package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SetClaudeModel returns Claude Code's settings.json content with the top-level
// "model" set to model. Every other byte is kept: an existing value is
// replaced in place and a missing key is inserted as the first member. Empty
// content stands for an absent file.
func SetClaudeModel(settings []byte, model string) ([]byte, error) {
	doc, err := jsonObjectDocument(settings)
	if err != nil {
		return nil, err
	}
	object, err := topLevelObject(doc)
	if err != nil {
		return nil, err
	}
	value, found, err := member(doc, object, "model")
	if err != nil {
		return nil, err
	}
	if !found {
		return insertMember(doc, object, "model", model), nil
	}
	var current string
	if json.Unmarshal(doc[value.start:value.end], &current) == nil && current == model {
		return doc, nil
	}
	return splice(doc, value.start, value.end, mustMarshal(model)), nil
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

// span is a byte range [start, end) of a JSON value inside a document.
type span struct{ start, end int }

// jsonObjectDocument returns settings, or an empty object when the file is
// absent or empty.
func jsonObjectDocument(content []byte) ([]byte, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return []byte("{}\n"), nil
	}
	if !json.Valid(content) {
		return nil, fmt.Errorf("not valid JSON")
	}
	return content, nil
}

func topLevelObject(doc []byte) (span, error) {
	start := len(doc) - len(bytes.TrimLeft(doc, " \t\r\n"))
	end := len(bytes.TrimRight(doc, " \t\r\n"))
	if doc[start] != '{' {
		return span{}, fmt.Errorf("not a JSON object")
	}
	return span{start, end}, nil
}

// member finds the value of key in the object at object.
func member(doc []byte, object span, key string) (span, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(doc[object.start:object.end]))
	if _, err := decoder.Token(); err != nil {
		return span{}, false, err
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return span{}, false, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return span{}, false, err
		}
		if name == key {
			end := object.start + int(decoder.InputOffset())
			return span{end - len(value), end}, true, nil
		}
	}
	return span{}, false, nil
}

// insertMember adds key as the first member of the object at object, matching
// the indentation of the member that follows it.
func insertMember(doc []byte, object span, key string, value any) []byte {
	member := mustMarshal(key) + ": "
	first := object.end - len(bytes.TrimLeft(doc[object.start+1:object.end], " \t\r\n"))
	if doc[first] == '}' {
		outer := lineIndent(doc, object.start)
		inner := outer + "  "
		return splice(doc, object.start+1, first, "\n"+inner+member+marshalIndented(value, inner)+"\n"+outer)
	}
	if !bytes.Contains(doc[object.start:first], []byte("\n")) {
		return splice(doc, first, first, member+mustMarshal(value)+", ")
	}
	indent := lineIndent(doc, first)
	return splice(doc, first, first, member+marshalIndented(value, indent)+",\n"+indent)
}

// appendElement adds value as the last element of the array at array,
// matching the indentation of its first element.
func appendElement(doc []byte, array span, value any) []byte {
	inside := doc[array.start+1 : array.end-1]
	first := array.end - 1 - len(bytes.TrimLeft(inside, " \t\r\n"))
	last := array.start + 1 + len(bytes.TrimRight(inside, " \t\r\n"))
	if first == array.end-1 {
		outer := lineIndent(doc, array.start)
		inner := outer + "  "
		return splice(doc, array.start+1, array.end-1, "\n"+inner+marshalIndented(value, inner)+"\n"+outer)
	}
	if !bytes.Contains(doc[array.start:first], []byte("\n")) {
		return splice(doc, last, last, ", "+mustMarshal(value))
	}
	indent := lineIndent(doc, first)
	return splice(doc, last, last, ",\n"+indent+marshalIndented(value, indent))
}

// lineIndent returns the leading whitespace of the line holding doc[at].
func lineIndent(doc []byte, at int) string {
	lineStart := bytes.LastIndexByte(doc[:at], '\n') + 1
	line := doc[lineStart:at]
	return string(line[:len(line)-len(bytes.TrimLeft(line, " \t"))])
}

func splice(doc []byte, start, end int, text string) []byte {
	out := make([]byte, 0, len(doc)+len(text))
	out = append(out, doc[:start]...)
	out = append(out, text...)
	return append(out, doc[end:]...)
}

// mustMarshal renders value as compact JSON.
func mustMarshal(value any) string {
	return encodeJSON(value, nil)
}

// marshalIndented renders value as JSON spread over lines indented from
// prefix, for a value that starts on a line indented by prefix.
func marshalIndented(value any, prefix string) string {
	return encodeJSON(value, func(encoder *json.Encoder) { encoder.SetIndent(prefix, "  ") })
}

func encodeJSON(value any, configure func(*json.Encoder)) string {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if configure != nil {
		configure(encoder)
	}
	if err := encoder.Encode(value); err != nil {
		panic(fmt.Sprintf("marshal %v: %v", value, err))
	}
	return strings.TrimSuffix(b.String(), "\n")
}
