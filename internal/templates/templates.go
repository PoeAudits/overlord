// Package templates embeds the Setup manifest and the Templates Setup writes
// into Projects.
package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Version is the Template version passed to every Template.
const Version = 1

// Strategy says how Setup guarantees one manifest item.
type Strategy string

const (
	Create  Strategy = "create"
	Dir     Strategy = "dir"
	Symlink Strategy = "symlink"
	Block   Strategy = "block"
	Labels  Strategy = "labels"
	// Generated renders each Role in the roles.yaml at Path into one agent
	// file per Harness.
	Generated Strategy = "generated"
)

// Item is one guaranteed path in a Project, relative to the Project root.
type Item struct {
	Path     string   `yaml:"path"`
	Strategy Strategy `yaml:"strategy"`
	// Template names the file in files/ that the create strategy renders.
	Template string `yaml:"template"`
	// SkipIfAny is a glob relative to the Project root; when anything matches
	// it, a create item counts as satisfied and is not written.
	SkipIfAny string `yaml:"skipIfAny"`
	// Target is the link destination for the symlink strategy.
	Target string `yaml:"target"`
	// Block names the Managed Block the block strategy owns in the file at
	// Path; its body is rendered from Template.
	Block string `yaml:"block"`
	// Language limits the item to Projects where it is detected (see
	// Languages); "any" means at least one detected language.
	Language string `yaml:"language"`
	// CreateWith names a Template appended after the block, outside it, only
	// when Setup creates the file.
	CreateWith string `yaml:"createWith"`
	// Content is the rendered agent file for a generated item. Setup fills it
	// in; the manifest never sets it.
	Content []byte `yaml:"-"`
	// Labels are the GitHub labels the labels strategy ensures on origin.
	Labels []Label `yaml:"labels"`
}

// Label is one GitHub issue label; Color is hex without the leading #.
type Label struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Color       string `yaml:"color"`
}

// Languages are the languages Setup detects, in the order Templates see them,
// each with the files in the Project root that mark it.
var Languages = []struct {
	Name    string
	Markers []string
}{
	{Name: "Go", Markers: []string{"go.mod"}},
	{Name: "TypeScript", Markers: []string{"package.json", "tsconfig.json"}},
	{Name: "Python", Markers: []string{"pyproject.toml", "requirements.txt"}},
	{Name: "Rust", Markers: []string{"Cargo.toml"}},
}

// Data is what every Template can refer to.
type Data struct {
	ProjectName string
	// Owner and Repo come from the GitHub origin remote and are empty without one.
	Owner   string
	Repo    string
	Version int
	// Languages holds the detected names from Languages, in that order.
	Languages []string
}

//go:embed manifest.yaml
var manifest []byte

//go:embed files
var files embed.FS

// Manifest returns the guaranteed items in the order Setup checks them.
func Manifest() ([]Item, error) {
	var items []Item
	if err := yaml.Unmarshal(manifest, &items); err != nil {
		return nil, fmt.Errorf("parse embedded manifest: %w", err)
	}
	return items, nil
}

// Render executes the named Template from files/ with data.
func Render(name string, data Data) ([]byte, error) {
	parsed, err := template.New(name).Option("missingkey=error").ParseFS(files, "files/"+name)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	return out.Bytes(), nil
}
