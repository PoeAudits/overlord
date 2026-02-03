package sync

import (
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
