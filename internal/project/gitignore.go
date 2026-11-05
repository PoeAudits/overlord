package project

import (
	"context"
	"fmt"
	"strings"

	"github.com/PoeAudits/overlord/internal/system"
	"github.com/PoeAudits/overlord/internal/templates"
)

// maxListed caps how many tracked files a .gitignore warning names, since a
// tracked node_modules/ can hold thousands.
const maxListed = 10

// trackedIgnored names the tracked files that the patterns in the item's
// .gitignore block ignore, or "" when there are none or git cannot list them
// (outside a repository). Setup only reports them; untracking is the
// Project's call.
func trackedIgnored(ctx context.Context, runner system.Runner, root string, item templates.Item, data templates.Data) (string, error) {
	body, err := templates.Render(item.Template, data)
	if err != nil {
		return "", err
	}
	args := []string{"ls-files", "--cached", "--ignored"}
	for _, line := range strings.Split(string(body), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			args = append(args, "--exclude="+line)
		}
	}
	output, err := runner.Run(ctx, root, "git", args...)
	if err != nil {
		return "", nil
	}
	tracked := strings.FieldsFunc(string(output), func(r rune) bool { return r == '\n' })
	if len(tracked) == 0 {
		return "", nil
	}
	listed := strings.Join(tracked[:min(len(tracked), maxListed)], ", ")
	if len(tracked) > maxListed {
		listed += fmt.Sprintf(" and %d more", len(tracked)-maxListed)
	}
	return "tracked files it ignores, left tracked: " + listed, nil
}
