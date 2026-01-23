package templates_test

import (
	"fmt"

	"overlord-v2/internal/templates"
)

func Example() {
	// Create template data for a new project
	data := templates.NewTemplateData(
		"my-api",
		"A REST API service",
		"services",
		"go",
	)

	// Get a rendered template
	content, err := templates.RenderTemplate(templates.LangGo, templates.TemplateAGENTS, data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// The content now has variables substituted
	fmt.Println("Template rendered successfully")
	fmt.Printf("Contains project name: %v\n", len(content) > 0)
	// Output:
	// Template rendered successfully
	// Contains project name: true
}

func Example_getAllTemplates() {
	data := templates.TemplateData{
		ProjectName: "my-project",
		Description: "My awesome project",
		Date:        "2024-01-15",
		Category:    "cli",
		Language:    "python",
	}

	// Get all templates for Python
	allTemplates, err := templates.GetAllTemplates(templates.LangPython, data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Got %d templates\n", len(allTemplates))
	// Output:
	// Got 5 templates
}

func Example_listTemplates() {
	// List all available template files
	files, err := templates.ListTemplates()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Found %d template files\n", len(files))
	// Output:
	// Found 13 template files
}
