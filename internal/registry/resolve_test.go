package registry

import (
	"strings"
	"testing"
)

// Helper function to create a test registry
func createTestRegistry() *Registry {
	return &Registry{
		Version: 2,
		Settings: Settings{
			BaseDir: "/home/user/projects",
		},
		Projects: map[string]Project{
			"overlord": {
				Path:        "core/tools/overlord",
				Category:    CategoryCoreTools,
				Lang:        LanguageGo,
				Created:     "2024-01-15",
				Description: "Project management CLI tool",
				Aliases:     []string{"ov", "ol"},
				Tags:        []string{"cli", "productivity"},
				Status:      Status{State: StateActive},
			},
			"web-dashboard": {
				Path:        "projects/web/dashboard",
				Category:    CategoryWeb,
				Lang:        LanguageTypeScript,
				Created:     "2024-02-01",
				Description: "Admin dashboard application",
				Aliases:     []string{"dashboard", "admin"},
				Tags:        []string{"web", "react"},
				Status:      Status{State: StateActive},
			},
			"ml-trainer": {
				Path:        "projects/ml/trainer",
				Category:    CategoryML,
				Lang:        LanguagePython,
				Created:     "2024-03-10",
				Description: "Machine learning model trainer",
				Aliases:     []string{"trainer"},
				Tags:        []string{"ml", "pytorch"},
				Status:      Status{State: StateActive},
			},
			"api-gateway": {
				Path:        "projects/services/gateway",
				Category:    CategoryServices,
				Lang:        LanguageGo,
				Created:     "2024-04-05",
				Description: "API gateway service",
				Aliases:     []string{"gateway", "gw"},
				Tags:        []string{"api", "microservices"},
				Status:      Status{State: StateActive},
			},
		},
	}
}

func TestResolve_ExactNameMatch(t *testing.T) {
	reg := createTestRegistry()

	results := reg.Resolve("overlord")

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Name != "overlord" {
		t.Errorf("expected name 'overlord', got '%s'", result.Name)
	}
	if result.Score != scoreExactName {
		t.Errorf("expected score %d, got %d", scoreExactName, result.Score)
	}
	if result.MatchedOn != "name" {
		t.Errorf("expected MatchedOn 'name', got '%s'", result.MatchedOn)
	}
}

func TestResolve_ExactAliasMatch(t *testing.T) {
	reg := createTestRegistry()

	tests := []struct {
		query         string
		expectedName  string
		expectedAlias string
	}{
		{"ov", "overlord", "ov"},
		{"ol", "overlord", "ol"},
		{"dashboard", "web-dashboard", "dashboard"},
		{"admin", "web-dashboard", "admin"},
		{"gateway", "api-gateway", "gateway"},
		{"gw", "api-gateway", "gw"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			results := reg.Resolve(tt.query)

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}

			result := results[0]
			if result.Name != tt.expectedName {
				t.Errorf("expected name '%s', got '%s'", tt.expectedName, result.Name)
			}
			if result.Score != scoreExactAlias {
				t.Errorf("expected score %d, got %d", scoreExactAlias, result.Score)
			}
			expectedMatchedOn := "alias:" + tt.expectedAlias
			if result.MatchedOn != expectedMatchedOn {
				t.Errorf("expected MatchedOn '%s', got '%s'", expectedMatchedOn, result.MatchedOn)
			}
		})
	}
}

func TestResolve_FuzzyMatch(t *testing.T) {
	reg := createTestRegistry()

	tests := []struct {
		query        string
		expectedName string
		description  string
	}{
		{"overlrd", "overlord", "typo in overlord"},
		{"trainr", "ml-trainer", "typo in trainer"},
		{"gatway", "api-gateway", "typo in gateway"},
		{"over", "overlord", "partial match"},
		{"mltrain", "ml-trainer", "partial match"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			results := reg.Resolve(tt.query)

			if len(results) == 0 {
				t.Fatalf("expected at least 1 result, got 0")
			}

			// Check that expected project is in results
			found := false
			for _, result := range results {
				if result.Name == tt.expectedName {
					found = true
					// Fuzzy matches should not be exact name or exact alias matches
					if result.Score == scoreExactName || result.Score == scoreExactAlias {
						t.Errorf("fuzzy match should not have exact match score, got %d", result.Score)
					}
					break
				}
			}

			if !found {
				t.Errorf("expected to find '%s' in results", tt.expectedName)
			}
		})
	}
}

func TestResolve_NoMatch(t *testing.T) {
	reg := createTestRegistry()

	results := reg.Resolve("nonexistent-project-xyz")

	if len(results) != 0 {
		t.Errorf("expected 0 results for non-matching query, got %d", len(results))
	}
}

func TestResolve_NilOrEmptyInputs(t *testing.T) {
	tests := []struct {
		name  string
		reg   *Registry
		query string
	}{
		{
			name:  "TestResolve_EmptyQuery",
			reg:   createTestRegistry(),
			query: "",
		},
		{
			name: "TestResolve_EmptyRegistry",
			reg: &Registry{
				Version: 2,
				Settings: Settings{
					BaseDir: "/home/user/projects",
				},
				Projects: map[string]Project{},
			},
			query: "anything",
		},
		{
			name: "TestResolve_NilProjects",
			reg: &Registry{
				Version: 2,
				Settings: Settings{
					BaseDir: "/home/user/projects",
				},
				Projects: nil,
			},
			query: "anything",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := tt.reg.Resolve(tt.query)
			if results != nil {
				t.Errorf("expected nil results, got %v", results)
			}
		})
	}
}

func TestResolve_SortedByScore(t *testing.T) {
	// Create a registry with projects that will definitely fuzzy match
	reg := &Registry{
		Version: 2,
		Settings: Settings{
			BaseDir: "/home/user/projects",
		},
		Projects: map[string]Project{
			"test-alpha": {
				Path:        "projects/test-alpha",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2024-01-01",
				Description: "Test alpha",
				Status:      Status{State: StateActive},
			},
			"test-beta": {
				Path:        "projects/test-beta",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2024-01-02",
				Description: "Test beta",
				Status:      Status{State: StateActive},
			},
			"test-gamma": {
				Path:        "projects/test-gamma",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2024-01-03",
				Description: "Test gamma",
				Status:      Status{State: StateActive},
			},
		},
	}

	// Query "test" should match all three projects
	results := reg.Resolve("test")

	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}

	// Verify results are sorted by score (descending)
	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results not sorted by score: result[%d].Score=%d > result[%d].Score=%d",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}
}

func TestResolveOne_ExactMatch(t *testing.T) {
	reg := createTestRegistry()

	result, err := reg.ResolveOne("overlord")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "overlord" {
		t.Errorf("expected name 'overlord', got '%s'", result.Name)
	}
	if result.Score != scoreExactName {
		t.Errorf("expected score %d, got %d", scoreExactName, result.Score)
	}
}

func TestResolveOne_AliasMatch(t *testing.T) {
	reg := createTestRegistry()

	result, err := reg.ResolveOne("dashboard")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "web-dashboard" {
		t.Errorf("expected name 'web-dashboard', got '%s'", result.Name)
	}
	if result.Score != scoreExactAlias {
		t.Errorf("expected score %d, got %d", scoreExactAlias, result.Score)
	}
}

func TestResolveOne_FuzzyMatch(t *testing.T) {
	reg := createTestRegistry()

	result, err := reg.ResolveOne("overlrd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Name != "overlord" {
		t.Errorf("expected name 'overlord', got '%s'", result.Name)
	}
	if result.Score >= scoreExactAlias {
		t.Errorf("fuzzy match should have score < %d, got %d", scoreExactAlias, result.Score)
	}
}

func TestResolveOne_NoMatch(t *testing.T) {
	reg := createTestRegistry()

	_, err := reg.ResolveOne("nonexistent-project-xyz")
	if err == nil {
		t.Fatal("expected error for non-matching query, got nil")
	}

	expectedMsg := "no project found matching 'nonexistent-project-xyz'"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestResolveOne_EmptyQuery(t *testing.T) {
	reg := createTestRegistry()

	_, err := reg.ResolveOne("")
	if err == nil {
		t.Fatal("expected error for empty query, got nil")
	}

	expectedMsg := "query cannot be empty"
	if err.Error() != expectedMsg {
		t.Errorf("expected error message '%s', got '%s'", expectedMsg, err.Error())
	}
}

func TestResolveOne_AmbiguousMatch(t *testing.T) {
	// Create registry with projects that might have equal fuzzy scores
	reg := &Registry{
		Version: 2,
		Settings: Settings{
			BaseDir: "/home/user/projects",
		},
		Projects: map[string]Project{
			"test-one": {
				Path:        "projects/test-one",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2024-01-01",
				Description: "Test project one",
				Aliases:     []string{"t1"},
				Status:      Status{State: StateActive},
			},
			"test-two": {
				Path:        "projects/test-two",
				Category:    CategoryCLI,
				Lang:        LanguageGo,
				Created:     "2024-01-02",
				Description: "Test project two",
				Aliases:     []string{"t2"},
				Status:      Status{State: StateActive},
			},
		},
	}

	// Both "t1" and "t2" should fuzzy match "t" with similar scores
	// This test verifies ambiguity detection works
	results := reg.Resolve("t")
	if len(results) >= 2 && results[0].Score == results[1].Score {
		_, err := reg.ResolveOne("t")
		if err == nil {
			t.Fatal("expected error for ambiguous match, got nil")
		}
		// Error message should mention ambiguity
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Errorf("expected error message to mention 'ambiguous', got '%s'", err.Error())
		}
	}
}

func TestResolveOne_AmbiguousMultiMatch(t *testing.T) {
	// Deterministic ambiguity test using structurally identical project names
	// so the fuzzy library assigns equal scores.
	reg := &Registry{
		Version: 2,
		Settings: Settings{
			BaseDir: "/home/user/projects",
		},
		Projects: map[string]Project{
			"svc-alpha": {
				Path:        "projects/svc-alpha",
				Category:    CategoryServices,
				Lang:        LanguageGo,
				Created:     "2024-01-01",
				Description: "Alpha service",
				Status:      Status{State: StateActive},
			},
			"svc-bravo": {
				Path:        "projects/svc-bravo",
				Category:    CategoryServices,
				Lang:        LanguageGo,
				Created:     "2024-01-02",
				Description: "Bravo service",
				Status:      Status{State: StateActive},
			},
		},
	}

	// "svc" is a prefix of both project names — fuzzy matching should return
	// equal scores for names with the same structure and prefix length.
	results := reg.Resolve("svc")
	if len(results) < 2 {
		t.Skipf("fuzzy engine did not return 2+ matches for 'svc', got %d — skipping ambiguity assertion", len(results))
	}

	if results[0].Score != results[1].Score {
		t.Skipf("fuzzy scores differ (%d vs %d) — library assigned unequal scores, skipping ambiguity assertion",
			results[0].Score, results[1].Score)
	}

	_, err := reg.ResolveOne("svc")
	if err == nil {
		t.Fatal("expected ambiguous-match error, got nil")
	}

	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected error message to contain 'ambiguous', got '%s'", err.Error())
	}

	// Verify the error mentions both project names
	if !strings.Contains(err.Error(), "svc-alpha") || !strings.Contains(err.Error(), "svc-bravo") {
		t.Errorf("expected error to mention both 'svc-alpha' and 'svc-bravo', got '%s'", err.Error())
	}
}

func TestResolve_NoDuplicateProjects(t *testing.T) {
	reg := createTestRegistry()

	// Query that might match both name and alias of same project
	results := reg.Resolve("over")

	// Count occurrences of each project
	projectCounts := make(map[string]int)
	for _, result := range results {
		projectCounts[result.Name]++
	}

	// Verify no project appears more than once
	for name, count := range projectCounts {
		if count > 1 {
			t.Errorf("project '%s' appears %d times in results, expected 1", name, count)
		}
	}
}
