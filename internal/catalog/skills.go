package catalog

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill is one skill directory in the agents directory.
type Skill struct {
	Name string
	// Dir is the absolute path of the skill directory.
	Dir string
}

// SkillsDir returns the directory under dir that holds the skills.
func SkillsDir(dir string) string {
	return filepath.Join(dir, "skills")
}

// InstructionsPath returns the global instructions file in dir.
func InstructionsPath(dir string) string {
	return filepath.Join(dir, "AGENTS.md")
}

// LoadSkills lists the skill directories under dir/skills, sorted by name.
// Hidden entries and plain files are not skills; a symlink counts when it
// resolves to a directory.
func LoadSkills(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(SkillsDir(dir))
	if err != nil {
		return nil, err
	}
	var skills []Skill
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(SkillsDir(dir), entry.Name())
		// DirEntry.IsDir reports the link itself, so a linked skill needs Stat.
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			continue
		}
		skills = append(skills, Skill{Name: entry.Name(), Dir: path})
	}
	return skills, nil
}
