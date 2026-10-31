// Package skills_test checks the skills Overlord ships for the agents catalog.
package skills_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/PoeAudits/overlord/internal/catalog"
	"github.com/PoeAudits/overlord/internal/cmd"
	"github.com/PoeAudits/overlord/internal/system"
)

func TestEverySkillHasFrontmatterNamedForItsDirectory(t *testing.T) {
	for _, skill := range loadSkills(t) {
		frontmatter := readFrontmatter(t, skill)
		if frontmatter.Name != skill.Name {
			t.Errorf("%s: frontmatter name = %q, want the directory name", skill.Name, frontmatter.Name)
		}
		if strings.TrimSpace(frontmatter.Description) == "" {
			t.Errorf("%s: frontmatter has no description", skill.Name)
		}
	}
}

// overlordCommand matches `overlord <subcommand>` wherever a skill names one.
var overlordCommand = regexp.MustCompile("\\boverlord ([a-z][a-z-]*)")

func TestEveryOverlordCommandASkillNamesExists(t *testing.T) {
	for _, skill := range loadSkills(t) {
		body := readSkill(t, skill)
		for _, match := range overlordCommand.FindAllStringSubmatch(string(body), -1) {
			var out bytes.Buffer
			sys := system.System{Home: t.TempDir(), Getenv: func(string) string { return "" }, Workdir: t.TempDir(), Stdout: &out, Stderr: &out}
			if code := cmd.Execute(sys, []string{match[1], "--help"}); code != 0 {
				t.Errorf("%s names `overlord %s`, which is not a command: %s", skill.Name, match[1], out.String())
			}
		}
	}
}

func loadSkills(t *testing.T) []catalog.Skill {
	t.Helper()
	// This directory is laid out like the agents catalog's skills directory.
	skills, err := catalog.LoadSkills("..")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) == 0 {
		t.Fatal("no skills found")
	}
	return skills
}

func readSkill(t *testing.T, skill catalog.Skill) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(skill.Dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func readFrontmatter(t *testing.T, skill catalog.Skill) frontmatter {
	t.Helper()
	body := string(readSkill(t, skill))
	rest, ok := strings.CutPrefix(body, "---\n")
	if !ok {
		t.Fatalf("%s: SKILL.md does not start with a frontmatter block", skill.Name)
	}
	block, _, ok := strings.Cut(rest, "\n---\n")
	if !ok {
		t.Fatalf("%s: frontmatter block is not closed", skill.Name)
	}
	var parsed frontmatter
	if err := yaml.Unmarshal([]byte(block), &parsed); err != nil {
		t.Fatalf("%s: frontmatter is not YAML: %v", skill.Name, err)
	}
	return parsed
}
