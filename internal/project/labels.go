package project

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// labelsStatus lists the labels on the GitHub origin repo and reports which
// of the item's labels are missing. Anything that stops the lookup is a
// warning, never an error, so Setup still applies every other item.
func labelsStatus(ctx context.Context, runner system.Runner, root string, item templates.Item, data templates.Data) (Status, string, []string) {
	if data.Owner == "" {
		return Warning, "no GitHub origin remote", nil
	}
	output, err := runner.Run(ctx, root, "gh", "label", "list", "--repo", data.Owner+"/"+data.Repo, "--json", "name", "--limit", "1000")
	if err != nil {
		return Warning, "gh label list failed: " + failure(output, err), nil
	}
	var existing []ghLabel
	if err := json.Unmarshal(output, &existing); err != nil {
		return Warning, "gh label list failed: unreadable output", nil
	}
	var missing []string
	for _, label := range item.Labels {
		if !slices.Contains(existing, ghLabel{Name: label.Name}) {
			missing = append(missing, label.Name)
		}
	}
	if len(missing) == 0 {
		return OK, "", nil
	}
	return Missing, strings.Join(missing, ", "), missing
}

// ghLabel is one entry of `gh label list --json name`.
type ghLabel struct {
	Name string `json:"name"`
}

// createLabels creates the labels row names as missing and returns the row,
// turned into a warning when gh fails.
func createLabels(ctx context.Context, runner system.Runner, root string, item templates.Item, data templates.Data, row Item) Item {
	for _, label := range item.Labels {
		if !slices.Contains(row.MissingLabels, label.Name) {
			continue
		}
		output, err := runner.Run(ctx, root, "gh", "label", "create", label.Name, "--repo", data.Owner+"/"+data.Repo, "--description", label.Description, "--color", label.Color)
		if err != nil {
			row.Status, row.Detail = Warning, "gh label create "+label.Name+" failed: "+failure(output, err)
			return row
		}
	}
	return row
}

// failure is the first line gh printed, else the process error.
func failure(output []byte, err error) string {
	line, _, _ := bytes.Cut(bytes.TrimSpace(output), []byte("\n"))
	if len(line) == 0 {
		return err.Error()
	}
	return string(line)
}
