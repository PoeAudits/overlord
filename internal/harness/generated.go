package harness

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
)

// FileState is how a file Overlord generates compares with what it would
// write now.
type FileState string

const (
	FileMissing FileState = "missing"
	// FileHandWritten is a file Overlord did not generate; it is never
	// replaced.
	FileHandWritten FileState = "hand-written"
	FileCurrent     FileState = "current"
	FileStale       FileState = "stale"
)

// CompareGenerated compares the file at path with want. isGenerated tells a
// file Overlord wrote (IsGenerated, IsGeneratedPlugin) from one the user
// wrote.
func CompareGenerated(path string, want []byte, isGenerated func([]byte) bool) (FileState, error) {
	current, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return FileMissing, nil
	}
	if err != nil {
		return "", err
	}
	if bytes.Equal(current, want) {
		return FileCurrent, nil
	}
	if !isGenerated(current) {
		return FileHandWritten, nil
	}
	return FileStale, nil
}
