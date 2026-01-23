package cmd

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "short string unchanged",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length unchanged",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "long string truncated",
			input:  "hello world this is a long string",
			maxLen: 15,
			want:   "hello world ...",
		},
		{
			name:   "very short maxLen",
			input:  "hello",
			maxLen: 3,
			want:   "...",
		},
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestHasAllTags(t *testing.T) {
	tests := []struct {
		name         string
		projectTags  []string
		requiredTags []string
		want         bool
	}{
		{
			name:         "all tags present",
			projectTags:  []string{"api", "backend", "go"},
			requiredTags: []string{"api", "backend"},
			want:         true,
		},
		{
			name:         "missing tag",
			projectTags:  []string{"api", "backend"},
			requiredTags: []string{"api", "frontend"},
			want:         false,
		},
		{
			name:         "empty required tags",
			projectTags:  []string{"api", "backend"},
			requiredTags: []string{},
			want:         true,
		},
		{
			name:         "empty project tags",
			projectTags:  []string{},
			requiredTags: []string{"api"},
			want:         false,
		},
		{
			name:         "both empty",
			projectTags:  []string{},
			requiredTags: []string{},
			want:         true,
		},
		{
			name:         "single tag match",
			projectTags:  []string{"cli"},
			requiredTags: []string{"cli"},
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasAllTags(tt.projectTags, tt.requiredTags)
			if got != tt.want {
				t.Errorf("hasAllTags(%v, %v) = %v, want %v", tt.projectTags, tt.requiredTags, got, tt.want)
			}
		})
	}
}
