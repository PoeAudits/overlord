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
	if query == "" || len(r.Projects) == 0 {
		return nil
	}

	if project, ok := r.Projects[query]; ok {
		return []ResolveResult{{
			Name:      query,
			Project:   project,
			Score:     scoreExactName,
			MatchedOn: "name",
		}}
	}

	for name, project := range r.Projects {
		for _, alias := range project.Aliases {
			if alias == query {
				return []ResolveResult{{
					Name:      name,
					Project:   project,
					Score:     scoreExactAlias,
					MatchedOn: fmt.Sprintf("alias:%s", alias),
				}}
			}
		}
	}

	results := fuzzyMatch(query, buildSearchTargets(r.Projects), r.Projects)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

func buildSearchTargets(projects map[string]Project) []searchTarget {
	targets := make([]searchTarget, 0, len(projects))
	for name, project := range projects {
		targets = append(targets, searchTarget{text: name, projectKey: name, matchType: "name"})
		for _, alias := range project.Aliases {
			targets = append(targets, searchTarget{
				text:       alias,
				projectKey: name,
				matchType:  fmt.Sprintf("alias:%s", alias),
			})
		}
	}
	return targets
}

func fuzzyMatch(query string, targets []searchTarget, projects map[string]Project) []ResolveResult {
	targetStrings := make([]string, len(targets))
	for i, t := range targets {
		targetStrings[i] = t.text
	}

	matches := fuzzy.Find(query, targetStrings)
	results := make([]ResolveResult, 0, len(matches))
	seenProjects := make(map[string]struct{}, len(matches))

	for _, match := range matches {
		if match.Score < scoreFuzzyMin {
			continue
		}

		target := targets[match.Index]
		if _, ok := seenProjects[target.projectKey]; ok {
			continue
		}

		seenProjects[target.projectKey] = struct{}{}
		results = append(results, ResolveResult{
			Name:      target.projectKey,
			Project:   projects[target.projectKey],
			Score:     match.Score,
			MatchedOn: target.matchType,
		})
	}

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
