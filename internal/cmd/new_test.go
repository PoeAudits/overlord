package cmd

import (
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

func TestGetLanguage(t *testing.T) {
	// Save original values
	origPy := newOpts.langPy
	origTs := newOpts.langTs
	origGo := newOpts.langGo
	origSol := newOpts.langSol

	// Restore after test
	defer func() {
		newOpts.langPy = origPy
		newOpts.langTs = origTs
		newOpts.langGo = origGo
		newOpts.langSol = origSol
	}()

	tests := []struct {
		name     string
		py       bool
		ts       bool
		goLang   bool
		sol      bool
		expected string
	}{
		{name: "default", py: false, ts: false, goLang: false, sol: false, expected: "base"},
		{name: "python", py: true, ts: false, goLang: false, sol: false, expected: "python"},
		{name: "typescript", py: false, ts: true, goLang: false, sol: false, expected: "typescript"},
		{name: "go", py: false, ts: false, goLang: true, sol: false, expected: "go"},
		{name: "solidity", py: false, ts: false, goLang: false, sol: true, expected: "solidity"},
		// First flag wins when multiple are set
		{name: "py wins over ts", py: true, ts: true, goLang: false, sol: false, expected: "python"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newOpts.langPy = tt.py
			newOpts.langTs = tt.ts
			newOpts.langGo = tt.goLang
			newOpts.langSol = tt.sol

			result := getLanguage()
			if string(result) != tt.expected {
				t.Errorf("getLanguage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "absolute path", input: "/home/user/project", wantErr: false},
		{name: "tilde path", input: "~/project", wantErr: false},
		{name: "tilde only", input: "~", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "tilde user", input: "~user/project", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := expandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("expandPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	t.Run("cleanup runs on failure", func(t *testing.T) {
		c := &cleanup{}
		c.add("/tmp/test1")
		c.add("/tmp/test2")

		// success is false by default, so cleanup should run
		// We can't easily test the actual removal without creating files,
		// but we can verify the structure
		if c.success {
			t.Error("cleanup.success should be false by default")
		}
		if len(c.paths) != 2 {
			t.Errorf("cleanup.paths should have 2 items, got %d", len(c.paths))
		}
	})

	t.Run("cleanup skipped on success", func(t *testing.T) {
		c := &cleanup{}
		c.add("/tmp/test1")
		c.success = true

		// When success is true, run() should not remove anything
		// This is a behavioral test - the paths should still be there
		c.run()
		if len(c.paths) != 1 {
			t.Errorf("cleanup.paths should still have 1 item after successful run, got %d", len(c.paths))
		}
	})
}
