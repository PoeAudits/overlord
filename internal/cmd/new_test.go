package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateName(t *testing.T) {
	// Create strings of specific lengths using strings.Repeat
	maxLengthName := "a" + strings.Repeat("b", 63) // 64 chars total (valid)
	tooLongName := "a" + strings.Repeat("b", 64)   // 65 chars total (invalid)

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid names
		{name: "simple name", input: "myproject", wantErr: false},
		{name: "with dash", input: "my-project", wantErr: false},
		{name: "with underscore", input: "my_project", wantErr: false},
		{name: "with numbers", input: "project123", wantErr: false},
		{name: "starts with number", input: "123project", wantErr: false},
		{name: "single char", input: "a", wantErr: false},
		{name: "mixed", input: "My-Project_123", wantErr: false},
		{name: "max length", input: maxLengthName, wantErr: false},

		// Invalid names
		{name: "empty", input: "", wantErr: true},
		{name: "starts with dash", input: "-project", wantErr: true},
		{name: "starts with underscore", input: "_project", wantErr: true},
		{name: "with space", input: "my project", wantErr: true},
		{name: "with dot", input: "my.project", wantErr: true},
		{name: "with special char", input: "my@project", wantErr: true},
		{name: "with slash", input: "my/project", wantErr: true},
		{name: "too long", input: tooLongName, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	t.Run("success=false removes files", func(t *testing.T) {
		dir := t.TempDir()
		fileA := filepath.Join(dir, "fileA.txt")
		fileB := filepath.Join(dir, "fileB.txt")

		if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
			t.Fatalf("failed to create fileA: %v", err)
		}
		if err := os.WriteFile(fileB, []byte("b"), 0o644); err != nil {
			t.Fatalf("failed to create fileB: %v", err)
		}

		c := &cleanup{}
		c.add(fileA)
		c.add(fileB)
		// success is false by default — run() should remove all paths
		c.run()

		if _, err := os.Stat(fileA); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("fileA still exists after cleanup with success=false")
		}
		if _, err := os.Stat(fileB); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("fileB still exists after cleanup with success=false")
		}
	})

	t.Run("success=true preserves files", func(t *testing.T) {
		dir := t.TempDir()
		fileA := filepath.Join(dir, "fileA.txt")

		if err := os.WriteFile(fileA, []byte("a"), 0o644); err != nil {
			t.Fatalf("failed to create fileA: %v", err)
		}

		c := &cleanup{}
		c.add(fileA)
		c.success = true
		// When success=true, run() must not delete anything
		c.run()

		if _, err := os.Stat(fileA); errors.Is(err, fs.ErrNotExist) {
			t.Errorf("fileA was deleted by cleanup even though success=true")
		}
	})
}
