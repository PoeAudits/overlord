package cmd

import (
	"testing"
)

func TestPadRight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "shorter than width",
			input:    "hello",
			width:    10,
			expected: "hello     ",
		},
		{
			name:     "equal to width",
			input:    "hello",
			width:    5,
			expected: "hello",
		},
		{
			name:     "longer than width",
			input:    "hello world",
			width:    5,
			expected: "hello world",
		},
		{
			name:     "empty string",
			input:    "",
			width:    5,
			expected: "     ",
		},
		{
			name:     "zero width",
			input:    "hello",
			width:    0,
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := padRight(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
			}
		})
	}
}

func TestPickerModelFilterProjects(t *testing.T) {
	items := []projectItem{
		{name: "overlord", category: "core-tools", description: "Project management"},
		{name: "dispatch", category: "core-agents", description: "Agent orchestration"},
		{name: "my-web-app", category: "web", description: "Frontend application"},
	}

	tests := []struct {
		name          string
		query         string
		expectedCount int
		expectedFirst string
	}{
		{
			name:          "empty query returns all",
			query:         "",
			expectedCount: 3,
			expectedFirst: "overlord",
		},
		{
			name:          "exact match",
			query:         "overlord",
			expectedCount: 1,
			expectedFirst: "overlord",
		},
		{
			name:          "partial match",
			query:         "over",
			expectedCount: 1,
			expectedFirst: "overlord",
		},
		{
			name:          "fuzzy match",
			query:         "disp",
			expectedCount: 1,
			expectedFirst: "dispatch",
		},
		{
			name:          "no match",
			query:         "zzzzz",
			expectedCount: 0,
			expectedFirst: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newPickerModel(items)
			m.textInput.SetValue(tt.query)
			m.filterProjects()

			if len(m.filtered) != tt.expectedCount {
				t.Errorf("filterProjects(%q) returned %d items, want %d", tt.query, len(m.filtered), tt.expectedCount)
			}

			if tt.expectedCount > 0 && m.filtered[0].name != tt.expectedFirst {
				t.Errorf("filterProjects(%q) first item = %q, want %q", tt.query, m.filtered[0].name, tt.expectedFirst)
			}
		})
	}
}

func TestPickerModelCursorBounds(t *testing.T) {
	items := []projectItem{
		{name: "project1", category: "cat1", description: "desc1"},
		{name: "project2", category: "cat2", description: "desc2"},
		{name: "project3", category: "cat3", description: "desc3"},
	}

	m := newPickerModel(items)

	// Cursor should start at 0
	if m.cursor != 0 {
		t.Errorf("initial cursor = %d, want 0", m.cursor)
	}

	// Filter to reduce items
	m.textInput.SetValue("project1")
	m.filterProjects()

	// Cursor should be reset to 0 when filter changes
	if m.cursor != 0 {
		t.Errorf("cursor after filter = %d, want 0", m.cursor)
	}

	// Cursor should not go below 0
	m.cursor = 0
	m.filterProjects()
	if m.cursor < 0 {
		t.Errorf("cursor went below 0: %d", m.cursor)
	}
}
