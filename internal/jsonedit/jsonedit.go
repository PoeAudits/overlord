// Package jsonedit edits JSON documents in place: it locates values by byte
// offsets and splices new text in, so every byte it does not change is kept.
package jsonedit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Span is a byte range [start, end) of a JSON value inside a document.
type Span struct{ Start, End int }

// Document returns content, or an empty object when the file is absent or
// empty.
func Document(content []byte) ([]byte, error) {
	if len(bytes.TrimSpace(content)) == 0 {
		return []byte("{}\n"), nil
	}
	if !json.Valid(content) {
		return nil, fmt.Errorf("not valid JSON")
	}
	return content, nil
}

// Object locates the top-level object of doc.
func Object(doc []byte) (Span, error) {
	start := len(doc) - len(bytes.TrimLeft(doc, " \t\r\n"))
	end := len(bytes.TrimRight(doc, " \t\r\n"))
	if doc[start] != '{' {
		return Span{}, fmt.Errorf("not a JSON object")
	}
	return Span{start, end}, nil
}

// Member finds the value of key in the object at object.
func Member(doc []byte, object Span, key string) (Span, bool, error) {
	decoder := json.NewDecoder(bytes.NewReader(doc[object.Start:object.End]))
	if _, err := decoder.Token(); err != nil {
		return Span{}, false, err
	}
	for decoder.More() {
		name, err := decoder.Token()
		if err != nil {
			return Span{}, false, err
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return Span{}, false, err
		}
		if name == key {
			end := object.Start + int(decoder.InputOffset())
			return Span{end - len(value), end}, true, nil
		}
	}
	return Span{}, false, nil
}

// InsertMember adds key as the first member of the object at object, matching
// the indentation of the member that follows it.
func InsertMember(doc []byte, object Span, key string, value any) []byte {
	member := Marshal(key) + ": "
	first := object.End - len(bytes.TrimLeft(doc[object.Start+1:object.End], " \t\r\n"))
	if doc[first] == '}' {
		outer := lineIndent(doc, object.Start)
		inner := outer + "  "
		return Splice(doc, Span{object.Start + 1, first}, "\n"+inner+member+MarshalIndented(value, inner)+"\n"+outer)
	}
	if !bytes.Contains(doc[object.Start:first], []byte("\n")) {
		return Splice(doc, Span{first, first}, member+Marshal(value)+", ")
	}
	indent := lineIndent(doc, first)
	return Splice(doc, Span{first, first}, member+MarshalIndented(value, indent)+",\n"+indent)
}

// AppendElement adds value as the last element of the array at array,
// matching the indentation of its first element.
func AppendElement(doc []byte, array Span, value any) []byte {
	inside := doc[array.Start+1 : array.End-1]
	first := array.End - 1 - len(bytes.TrimLeft(inside, " \t\r\n"))
	last := array.Start + 1 + len(bytes.TrimRight(inside, " \t\r\n"))
	if first == array.End-1 {
		outer := lineIndent(doc, array.Start)
		inner := outer + "  "
		return Splice(doc, Span{array.Start + 1, array.End - 1}, "\n"+inner+MarshalIndented(value, inner)+"\n"+outer)
	}
	if !bytes.Contains(doc[array.Start:first], []byte("\n")) {
		return Splice(doc, Span{last, last}, ", "+Marshal(value))
	}
	indent := lineIndent(doc, first)
	return Splice(doc, Span{last, last}, ",\n"+indent+MarshalIndented(value, indent))
}

// lineIndent returns the leading whitespace of the line holding doc[at].
func lineIndent(doc []byte, at int) string {
	lineStart := bytes.LastIndexByte(doc[:at], '\n') + 1
	line := doc[lineStart:at]
	return string(line[:len(line)-len(bytes.TrimLeft(line, " \t"))])
}

// Splice replaces the bytes at at with text.
func Splice(doc []byte, at Span, text string) []byte {
	out := make([]byte, 0, len(doc)+len(text))
	out = append(out, doc[:at.Start]...)
	out = append(out, text...)
	return append(out, doc[at.End:]...)
}

// Marshal renders value as compact JSON.
func Marshal(value any) string {
	return encodeJSON(value, nil)
}

// MarshalIndented renders value as JSON spread over lines indented from
// prefix, for a value that starts on a line indented by prefix.
func MarshalIndented(value any, prefix string) string {
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
