package sync

import (
	"fmt"
	"testing"
)

func TestConflictType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		ct       ConflictType
		expected bool
	}{
		{"local-only valid", ConflictLocalOnly, true},
		{"remote-only valid", ConflictRemoteOnly, true},
		{"both-modified valid", ConflictBothModified, true},
		{"invalid type", ConflictType("invalid"), false},
		{"empty type", ConflictType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ct.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestConflictType_String(t *testing.T) {
	tests := []struct {
		name     string
		ct       ConflictType
		expected string
	}{
		{"local-only", ConflictLocalOnly, "Local only"},
		{"remote-only", ConflictRemoteOnly, "Remote only"},
		{"both-modified", ConflictBothModified, "Modified on both sides"},
		{"unknown", ConflictType("invalid"), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ct.String()
			if result != tt.expected {
				t.Errorf("String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConflict_Validate(t *testing.T) {
	tests := []struct {
		name     string
		conflict Conflict
		wantErr  bool
	}{
		{
			name: "valid conflict",
			conflict: Conflict{
				Path: "file.txt",
				Type: ConflictLocalOnly,
			},
			wantErr: false,
		},
		{
			name: "empty path",
			conflict: Conflict{
				Path: "",
				Type: ConflictLocalOnly,
			},
			wantErr: true,
		},
		{
			name: "invalid type",
			conflict: Conflict{
				Path: "file.txt",
				Type: ConflictType("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conflict.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRsyncOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name: "simple file list",
			output: `sending incremental file list
file1.txt
file2.go
dir/file3.md

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec
total size is 10,000  speedup is 7.75`,
			expected: []string{"file1.txt", "file2.go", "dir/file3.md"},
		},
		{
			name: "with directories",
			output: `sending incremental file list
./
dir1/
dir1/file1.txt
dir2/
dir2/file2.go

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expected: []string{"dir1/file1.txt", "dir2/file2.go"},
		},
		{
			name: "empty output",
			output: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec
total size is 0  speedup is 0.00`,
			expected: []string{},
		},
		{
			name: "no changes",
			output: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			expected: []string{},
		},
		{
			name: "with deletion markers",
			output: `sending incremental file list
deleting old-file.txt
new-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expected: []string{"new-file.txt"},
		},
		{
			name: "nested directories",
			output: `sending incremental file list
src/
src/main.go
src/utils/
src/utils/helper.go
test/
test/main_test.go

sent 5,000 bytes  received 100 bytes  10,200.00 bytes/sec`,
			expected: []string{"src/main.go", "src/utils/helper.go", "test/main_test.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRsyncOutput(tt.output)
			if len(result) != len(tt.expected) {
				t.Errorf("parseRsyncOutput() returned %d files, want %d\nGot: %v\nWant: %v",
					len(result), len(tt.expected), result, tt.expected)
				return
			}
			for i, file := range result {
				if file != tt.expected[i] {
					t.Errorf("parseRsyncOutput()[%d] = %q, want %q", i, file, tt.expected[i])
				}
			}
		})
	}
}

func TestDetectConflictsFromFileLists(t *testing.T) {
	tests := []struct {
		name      string
		pushFiles []string
		pullFiles []string
		expected  []Conflict
	}{
		{
			name:      "no conflicts",
			pushFiles: []string{},
			pullFiles: []string{},
			expected:  []Conflict{},
		},
		{
			name:      "local-only changes",
			pushFiles: []string{"file1.txt", "file2.go"},
			pullFiles: []string{},
			expected: []Conflict{
				{Path: "file1.txt", Type: ConflictLocalOnly},
				{Path: "file2.go", Type: ConflictLocalOnly},
			},
		},
		{
			name:      "remote-only changes",
			pushFiles: []string{},
			pullFiles: []string{"file3.md", "file4.rs"},
			expected: []Conflict{
				{Path: "file3.md", Type: ConflictRemoteOnly},
				{Path: "file4.rs", Type: ConflictRemoteOnly},
			},
		},
		{
			name:      "both modified",
			pushFiles: []string{"file1.txt"},
			pullFiles: []string{"file1.txt"},
			expected: []Conflict{
				{Path: "file1.txt", Type: ConflictBothModified},
			},
		},
		{
			name:      "mixed conflicts",
			pushFiles: []string{"local.txt", "both.txt"},
			pullFiles: []string{"remote.txt", "both.txt"},
			expected: []Conflict{
				{Path: "both.txt", Type: ConflictBothModified},
				{Path: "local.txt", Type: ConflictLocalOnly},
				{Path: "remote.txt", Type: ConflictRemoteOnly},
			},
		},
		{
			name:      "multiple both-modified",
			pushFiles: []string{"file1.txt", "file2.go", "file3.md"},
			pullFiles: []string{"file1.txt", "file2.go", "file3.md"},
			expected: []Conflict{
				{Path: "file1.txt", Type: ConflictBothModified},
				{Path: "file2.go", Type: ConflictBothModified},
				{Path: "file3.md", Type: ConflictBothModified},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectConflictsFromFileLists(tt.pushFiles, tt.pullFiles)

			// Create maps for easier comparison (order doesn't matter)
			resultMap := make(map[string]ConflictType)
			for _, c := range result {
				resultMap[c.Path] = c.Type
			}

			expectedMap := make(map[string]ConflictType)
			for _, c := range tt.expected {
				expectedMap[c.Path] = c.Type
			}

			if len(resultMap) != len(expectedMap) {
				t.Errorf("detectConflictsFromFileLists() returned %d conflicts, want %d\nGot: %v\nWant: %v",
					len(resultMap), len(expectedMap), result, tt.expected)
				return
			}

			for path, expectedType := range expectedMap {
				resultType, ok := resultMap[path]
				if !ok {
					t.Errorf("expected conflict for %q not found", path)
					continue
				}
				if resultType != expectedType {
					t.Errorf("conflict for %q: got type %q, want %q", path, resultType, expectedType)
				}
			}
		})
	}
}

// ConflictMockExecutor for testing DetectConflicts
// This is separate from the MockExecutor in rsync_test.go because it needs
// different behavior (returning different outputs for push vs pull)
type ConflictMockExecutor struct {
	PushOutput string
	PullOutput string
	CallCount  int
}

func (m *ConflictMockExecutor) Execute(name string, args ...string) ([]byte, int, error) {
	m.CallCount++

	// Determine if this is a push or pull based on arguments
	// Push: local path comes before remote path
	// Pull: remote path comes before local path
	var output string
	if len(args) >= 2 {
		source := args[len(args)-2]
		// If source contains ":", it's a remote path (pull)
		if IsRemotePath(source) {
			output = m.PullOutput
		} else {
			output = m.PushOutput
		}
	}

	return []byte(output), 0, nil
}

func TestDetectConflictsWithRsync(t *testing.T) {
	tests := []struct {
		name       string
		pushOutput string
		pullOutput string
		expected   []Conflict
	}{
		{
			name: "no conflicts",
			pushOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			pullOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			expected: []Conflict{},
		},
		{
			name: "local-only file",
			pushOutput: `sending incremental file list
new-local-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			pullOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			expected: []Conflict{
				{Path: "new-local-file.txt", Type: ConflictLocalOnly},
			},
		},
		{
			name: "remote-only file",
			pushOutput: `sending incremental file list

sent 100 bytes  received 20 bytes  240.00 bytes/sec`,
			pullOutput: `sending incremental file list
new-remote-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expected: []Conflict{
				{Path: "new-remote-file.txt", Type: ConflictRemoteOnly},
			},
		},
		{
			name: "both modified",
			pushOutput: `sending incremental file list
modified-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			pullOutput: `sending incremental file list
modified-file.txt

sent 1,234 bytes  received 56 bytes  2,580.00 bytes/sec`,
			expected: []Conflict{
				{Path: "modified-file.txt", Type: ConflictBothModified},
			},
		},
		{
			name: "complex scenario",
			pushOutput: `sending incremental file list
local-only.txt
both-modified.go
src/
src/local-file.rs

sent 5,000 bytes  received 100 bytes  10,200.00 bytes/sec`,
			pullOutput: `sending incremental file list
remote-only.md
both-modified.go
docs/
docs/remote-file.txt

sent 5,000 bytes  received 100 bytes  10,200.00 bytes/sec`,
			expected: []Conflict{
				{Path: "both-modified.go", Type: ConflictBothModified},
				{Path: "local-only.txt", Type: ConflictLocalOnly},
				{Path: "src/local-file.rs", Type: ConflictLocalOnly},
				{Path: "remote-only.md", Type: ConflictRemoteOnly},
				{Path: "docs/remote-file.txt", Type: ConflictRemoteOnly},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &ConflictMockExecutor{
				PushOutput: tt.pushOutput,
				PullOutput: tt.pullOutput,
			}

			rsync := NewRsyncWithExecutor(mockExec)

			result, err := DetectConflictsWithRsync(
				rsync,
				"/local/path",
				"remote-host",
				"/remote/path",
				[]string{},
			)

			if err != nil {
				t.Fatalf("DetectConflictsWithRsync() error = %v", err)
			}

			// Create maps for easier comparison
			resultMap := make(map[string]ConflictType)
			for _, c := range result {
				resultMap[c.Path] = c.Type
			}

			expectedMap := make(map[string]ConflictType)
			for _, c := range tt.expected {
				expectedMap[c.Path] = c.Type
			}

			if len(resultMap) != len(expectedMap) {
				t.Errorf("DetectConflictsWithRsync() returned %d conflicts, want %d\nGot: %v\nWant: %v",
					len(resultMap), len(expectedMap), result, tt.expected)
				return
			}

			for path, expectedType := range expectedMap {
				resultType, ok := resultMap[path]
				if !ok {
					t.Errorf("expected conflict for %q not found", path)
					continue
				}
				if resultType != expectedType {
					t.Errorf("conflict for %q: got type %q, want %q", path, resultType, expectedType)
				}
			}

			// Verify rsync was called twice (push and pull)
			if mockExec.CallCount != 2 {
				t.Errorf("expected 2 rsync calls, got %d", mockExec.CallCount)
			}
		})
	}
}

func TestDetectConflictsWithRsync_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		localPath  string
		remoteHost string
		remotePath string
		wantErr    bool
	}{
		{
			name:       "valid paths",
			localPath:  "/local/path",
			remoteHost: "remote-host",
			remotePath: "/remote/path",
			wantErr:    false,
		},
		{
			name:       "empty local path",
			localPath:  "",
			remoteHost: "remote-host",
			remotePath: "/remote/path",
			wantErr:    true,
		},
		{
			name:       "empty remote host",
			localPath:  "/local/path",
			remoteHost: "",
			remotePath: "/remote/path",
			wantErr:    true,
		},
		{
			name:       "empty remote path",
			localPath:  "/local/path",
			remoteHost: "remote-host",
			remotePath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := &ConflictMockExecutor{
				PushOutput: "sending incremental file list\n",
				PullOutput: "sending incremental file list\n",
			}

			rsync := NewRsyncWithExecutor(mockExec)

			_, err := DetectConflictsWithRsync(
				rsync,
				tt.localPath,
				tt.remoteHost,
				tt.remotePath,
				[]string{},
			)

			if (err != nil) != tt.wantErr {
				t.Errorf("DetectConflictsWithRsync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDetectConflictsWithRsync_RsyncError(t *testing.T) {
	// Test error handling when rsync fails
	errorExec := &errorExecutor{
		err: fmt.Errorf("rsync command failed"),
	}

	rsync := NewRsyncWithExecutor(errorExec)

	_, err := DetectConflictsWithRsync(
		rsync,
		"/local/path",
		"remote-host",
		"/remote/path",
		[]string{},
	)

	if err == nil {
		t.Error("expected error when rsync fails, got nil")
	}
}

// errorExecutor always returns an error
type errorExecutor struct {
	err error
}

func (e *errorExecutor) Execute(name string, args ...string) ([]byte, int, error) {
	return nil, -1, e.err
}
