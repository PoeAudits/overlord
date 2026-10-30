package cmd_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const (
	labelList       = "gh label list --repo PoeAudits/sample --json name --limit 1000"
	allTriageLabels = `[{"name":"needs-triage"},{"name":"needs-info"},{"name":"ready-for-agent"},{"name":"ready-for-human"},{"name":"wontfix"}]`
)

func TestSetupCreatesOnlyTheMissingTriageLabels(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	runner := &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
		labelList:                   `[{"name":"bug"},{"name":"needs-info"},{"name":"ready-for-agent"},{"name":"wontfix"}]`,
		"gh label create needs-triage --repo PoeAudits/sample --description Maintainer needs to evaluate this issue --color e4e669": "",
		"gh label create ready-for-human --repo PoeAudits/sample --description Requires human implementation --color 1d76db":        "",
	}}
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := ghCalls(runner), []string{
		project + ": " + labelList,
		project + ": gh label create needs-triage --repo PoeAudits/sample --description Maintainer needs to evaluate this issue --color e4e669",
		project + ": gh label create ready-for-human --repo PoeAudits/sample --description Requires human implementation --color 1d76db",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gh commands:\n%q\nwant\n%q", got, want)
	}
	if want := "  created    triage labels (needs-triage, ready-for-human)\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

func TestSetupCreatesEveryTriageLabelWithItsDescriptionAndColour(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	runner := &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "https://github.com/PoeAudits/sample\n",
		labelList:                   `[]`,
		"gh label create needs-triage --repo PoeAudits/sample --description Maintainer needs to evaluate this issue --color e4e669":    "",
		"gh label create needs-info --repo PoeAudits/sample --description Waiting on reporter for more information --color d876e3":     "",
		"gh label create ready-for-agent --repo PoeAudits/sample --description Fully specified, ready for an AFK agent --color 0e8a16": "",
		"gh label create ready-for-human --repo PoeAudits/sample --description Requires human implementation --color 1d76db":           "",
		"gh label create wontfix --repo PoeAudits/sample --description Will not be actioned --color ffffff":                            "",
	}}
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got := len(ghCalls(runner)); got != 6 {
		t.Fatalf("gh commands = %q, want one list and five creates", ghCalls(runner))
	}
	if want := "  created    triage labels (needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix)\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

func TestSetupMakesNoCreateCallWhenEveryTriageLabelExists(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	runner := &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
		labelList:                   `[{"name":"wontfix"},{"name":"ready-for-human"},{"name":"ready-for-agent"},{"name":"needs-info"},{"name":"needs-triage"}]`,
	}}
	sys.Runner = runner

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := ghCalls(runner), []string{project + ": " + labelList}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gh commands = %q, want %q", got, want)
	}
	if want := "  ok         triage labels\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

func TestSetupCheckReportsTheMissingTriageLabelsWithoutCreatingAny(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	runner := &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
		labelList:                   `[{"name":"needs-triage"},{"name":"ready-for-agent"},{"name":"wontfix"}]`,
	}}
	sys.Runner = runner

	stdout, code := run(t, sys, "setup", "--check")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if got, want := ghCalls(runner), []string{project + ": " + labelList}; !reflect.DeepEqual(got, want) {
		t.Fatalf("gh commands = %q, want only the list", got)
	}
	if want := "  missing    triage labels (needs-info, ready-for-human)\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

func TestSetupCheckJSONListsTheMissingTriageLabels(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
		labelList:                   `[{"name":"needs-triage"},{"name":"needs-info"},{"name":"ready-for-agent"}]`,
	}}

	stdout, code := run(t, sys, "setup", "--check", "--json")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := `{"path":"triage labels","strategy":"labels","status":"missing","detail":"ready-for-human, wontfix","missingLabels":["ready-for-human","wontfix"]}`
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(stdout)); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !strings.Contains(compact.String(), want) {
		t.Fatalf("plan JSON lacks %s:\n%s", want, stdout)
	}
}

func TestSetupWarnsAboutTriageLabelsAndStillAppliesEverythingElse(t *testing.T) {
	tests := []struct {
		name    string
		outputs map[string]string
		detail  string
	}{
		{
			name:    "no origin remote",
			outputs: map[string]string{},
			detail:  "no GitHub origin remote",
		},
		{
			name:    "origin is not on GitHub",
			outputs: map[string]string{"git remote get-url origin": "git@gitlab.com:PoeAudits/sample.git\n"},
			detail:  "no GitHub origin remote",
		},
		{
			// The fake runner fails every command it has no output for, the
			// way gh does when it is not logged in or offline.
			name:    "gh fails",
			outputs: map[string]string{"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n"},
			detail:  "gh label list failed: error: No such remote 'origin'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := t.TempDir()
			sys := newSystem(t, project)
			runner := &fakeRunner{outputs: tt.outputs}
			sys.Runner = runner

			stdout, code := run(t, sys, "setup")

			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			if _, ok := tree(t, project)["GLOSSARY.md"]; !ok {
				t.Fatal("GLOSSARY.md not created beside the labels warning")
			}
			if want := "  warning    triage labels (" + tt.detail + ")\n"; !strings.HasSuffix(stdout, want) {
				t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
			}
			for _, call := range ghCalls(runner) {
				if strings.Contains(call, "gh label create") {
					t.Fatalf("created a label after a warning: %q", call)
				}
			}
		})
	}
}

func TestSetupWarnsWhenCreatingATriageLabelFails(t *testing.T) {
	project := t.TempDir()
	sys := newSystem(t, project)
	sys.Runner = &fakeRunner{outputs: map[string]string{
		"git remote get-url origin": "git@github.com:PoeAudits/sample.git\n",
		labelList:                   `[{"name":"needs-triage"},{"name":"needs-info"},{"name":"ready-for-agent"},{"name":"ready-for-human"}]`,
	}}

	stdout, code := run(t, sys, "setup")

	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if want := "  warning    triage labels (gh label create wontfix failed: error: No such remote 'origin')\n"; !strings.HasSuffix(stdout, want) {
		t.Fatalf("stdout does not end with %q:\n%s", want, stdout)
	}
}

// ghCalls keeps the recorded gh commands, dropping the git remote lookup.
func ghCalls(runner *fakeRunner) []string {
	var calls []string
	for _, call := range runner.calls {
		if strings.Contains(call, ": gh ") {
			calls = append(calls, call)
		}
	}
	return calls
}
