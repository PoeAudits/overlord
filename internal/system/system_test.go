package system_test

import (
	"context"
	"testing"

	"github.com/PoeAudits/overlord/internal/system"
)

func TestExecRunnerReturnsOnlyStdoutAndPutsStderrInTheError(t *testing.T) {
	output, err := system.ExecRunner{}.Run(context.Background(), t.TempDir(), "sh", "-c", `echo '[]'; echo 'warning: noise' >&2`)
	if err != nil || string(output) != "[]\n" {
		t.Fatalf("success: output %q, err %v; want only stdout", output, err)
	}

	output, err = system.ExecRunner{}.Run(context.Background(), t.TempDir(), "sh", "-c", `echo partial; echo 'HTTP 404: Not Found' >&2; exit 1`)
	if string(output) != "partial\n" || err == nil || err.Error() != "exit status 1: HTTP 404: Not Found" {
		t.Fatalf("failure: output %q, err %v; want stdout, and stderr in the error", output, err)
	}
}
