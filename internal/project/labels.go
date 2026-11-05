package project

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// labelListAttempts and labelListDelay bound the retry a non-nil sleep turns
// on: GitHub can refuse a label lookup for a repository it created moments ago.
const (
	labelListAttempts = 4
	labelListDelay    = 2 * time.Second
)

// labelsStatus lists the labels on the GitHub origin repo and reports which
// of the item's labels are missing. Anything that stops the lookup is a
// warning, never an error, so Setup still applies every other item. With a
// non-nil sleep a failing lookup is retried before it becomes a warning.
func labelsStatus(ctx context.Context, runner system.Runner, sleep func(time.Duration), root string, item templates.Item, data templates.Data) (Status, string, []string) {
	if data.Owner == "" {
		return Warning, "no GitHub origin remote", nil
	}
	var output []byte
	var err error
	for attempt := 1; ; attempt++ {
		output, err = runner.Run(ctx, root, "gh", "label", "list", "--repo", data.Owner+"/"+data.Repo, "--json", "name", "--limit", "1000")
		if err == nil || sleep == nil || attempt == labelListAttempts {
			break
		}
		sleep(labelListDelay)
	}
	if err != nil {
		return Warning, "gh label list failed: " + failure(output, err) + uncheckedLabels, nil
	}
	var existing []ghLabel
	if err := json.Unmarshal(output, &existing); err != nil {
		return Warning, "gh label list failed: unreadable output" + uncheckedLabels, nil
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

// uncheckedLabels ends a warning about a label lookup that failed.
const uncheckedLabels = "; triage labels not checked and may be missing, so the issue triage workflow may not work until overlord setup is re-run"

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
			row.Status, row.Detail = Warning, "gh label create "+label.Name+" failed: "+failure(output, err)+"; triage labels are missing, so the issue triage workflow will not work until overlord setup is re-run"
			return row
		}
	}
	return row
}

// failure is the first line a command printed to standard output, else the
// first line of its error, which holds what it printed to standard error.
func failure(output []byte, err error) string {
	line, _, _ := bytes.Cut(bytes.TrimSpace(output), []byte("\n"))
	if len(line) > 0 {
		return string(line)
	}
	first, _, _ := strings.Cut(err.Error(), "\n")
	return first
}
