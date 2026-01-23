package registry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

const (
	// Score constants for different match types
	scoreExactName  = 1000
	scoreExactAlias = 900
	scoreFuzzyMin   = 50 // Minimum fuzzy score to include
)

// ResolveResult represents a matched project
type ResolveResult struct {
	Name      string  // Project name in registry
	Project   Project // The project
	Score     int     // Match score (higher = better)
	MatchedOn string  // "name", "alias:<alias>", or "fuzzy"
}

// searchTarget represents a searchable string with metadata
type searchTarget struct {
	text       string // The searchable text
	projectKey string // The project name in registry
	matchType  string // "name" or "alias:<alias>"
}

// Resolve finds projects matching the query
// Returns matches sorted by score (best first)
func (r *Registry) Resolve(query string) []ResolveResult {
	if query == "" {
		return nil
	}

	if r.Projects == nil || len(r.Projects) == 0 {
		return nil
	}

	var results []ResolveResult

	// Step 1: Check for exact name match
	if project, ok := r.Projects[query]; ok {
		results = append(results, ResolveResult{
			Name:      query,
			Project:   project,
			Score:     scoreExactName,
			MatchedOn: "name",
		})
		return results // Exact match found, return immediately
	}

	// Step 2: Check for exact alias match
	for name, project := range r.Projects {
		for _, alias := range project.Aliases {
			if alias == query {
				results = append(results, ResolveResult{
					Name:      name,
					Project:   project,
					Score:     scoreExactAlias,
					MatchedOn: fmt.Sprintf("alias:%s", alias),
				})
				return results // Exact alias match found, return immediately
			}
		}
	}

	// Step 3: Fuzzy matching
	// Build list of searchable targets
	var targets []searchTarget
	for name, project := range r.Projects {
		// Add project name
		targets = append(targets, searchTarget{
			text:       name,
			projectKey: name,
			matchType:  "name",
		})

		// Add aliases
		for _, alias := range project.Aliases {
			targets = append(targets, searchTarget{
				text:       alias,
				projectKey: name,
				matchType:  fmt.Sprintf("alias:%s", alias),
			})
		}
	}

	// Extract text for fuzzy matching
	targetStrings := make([]string, len(targets))
	for i, t := range targets {
		targetStrings[i] = t.text
	}

	// Perform fuzzy match
	matches := fuzzy.Find(query, targetStrings)

	// Convert fuzzy matches to ResolveResults
	seenProjects := make(map[string]bool) // Track projects we've already added
	for _, match := range matches {
		if match.Score < scoreFuzzyMin {
			continue // Skip low-quality matches
		}

		target := targets[match.Index]

		// Skip if we've already added this project (prefer first/best match)
		if seenProjects[target.projectKey] {
			continue
		}
		seenProjects[target.projectKey] = true

		project := r.Projects[target.projectKey]
		results = append(results, ResolveResult{
			Name:      target.projectKey,
			Project:   project,
			Score:     match.Score,
			MatchedOn: target.matchType,
		})
	}

	// Sort by score (highest first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// ResolveOne finds the single best match
// Returns error if no match or ambiguous (multiple equal scores)
func (r *Registry) ResolveOne(query string) (ResolveResult, error) {
	if query == "" {
		return ResolveResult{}, fmt.Errorf("query cannot be empty")
	}

	results := r.Resolve(query)

	if len(results) == 0 {
		return ResolveResult{}, fmt.Errorf("no project found matching '%s'", query)
	}

	// Check for ambiguous results (multiple matches with same top score)
	if len(results) > 1 && results[0].Score == results[1].Score {
		// Build list of ambiguous matches
		var ambiguous []string
		topScore := results[0].Score
		for _, result := range results {
			if result.Score != topScore {
				break
			}
			ambiguous = append(ambiguous, result.Name)
		}

		return ResolveResult{}, fmt.Errorf("ambiguous match for '%s': multiple projects match equally: %s",
			query, strings.Join(ambiguous, ", "))
	}

	return results[0], nil
}
