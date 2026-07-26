package tmux

import (
	"errors"
	"strings"
	"testing"

	"github.com/PoeAudits/overlord/internal/checkpoint"
)

type checkpointExitError int

func (e checkpointExitError) Error() string { return "tmux exited" }
func (e checkpointExitError) ExitCode() int { return int(e) }

func TestCaptureCheckpointRecordsTopologyAndSkipsStartServer(t *testing.T) {
	t.Setenv("TMUX", "socket,1,0")
	var calls []string
	runner := func(_ string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		switch args[0] {
		case "display-message":
			return []byte("work\n"), nil
		case "list-sessions":
			return []byte("StartServer\nwork\n"), nil
		case "list-windows":
			return []byte("2\tcode\t1\tabcd,80x24,0,0,80x24,0,0\n"), nil
		case "list-panes":
			return []byte("4\t1\t/tmp/work\n"), nil
		}
		return nil, nil
	}
	cp, err := CaptureCheckpoint(runner)
	if err != nil {
		t.Fatal(err)
	}
	if cp.AttachSession != "work" || len(cp.Sessions) != 1 || cp.Sessions[0].Name != "work" {
		t.Fatalf("sessions = %#v", cp)
	}
	w := cp.Sessions[0].Windows[0]
	if w.Index != 2 || w.ActivePane != 4 || w.Panes[0].Path != "/tmp/work" || w.Layout == "" {
		t.Fatalf("window = %#v", w)
	}
	for _, call := range calls {
		if strings.Contains(call, "capture-pane") || strings.Contains(call, "show-environment") {
			t.Fatalf("captured prohibited data with %q", call)
		}
	}
}

func TestRestoreCheckpointCreatesMissingSessionsBySavedIndex(t *testing.T) {
	var created []string
	runner := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "has-session":
			return nil, checkpointExitError(1)
		case "new-session":
			created = append(created, args[3])
			return nil, nil
		case "list-windows":
			return []byte("0\n"), nil
		case "select-window", "select-pane":
			return nil, nil
		default:
			return nil, errors.New("unexpected tmux command: " + args[0])
		}
	}
	cp := &checkpoint.Checkpoint{Sessions: []checkpoint.Session{
		{Name: "third", Index: 3, Windows: []checkpoint.Window{{Name: "base"}}},
		{Name: "first", Index: 1, Windows: []checkpoint.Window{{Name: "base"}}},
		{Name: "second", Index: 2, Windows: []checkpoint.Window{{Name: "base"}}},
	}}
	RestoreCheckpoint(cp, runner, nil)
	if got, want := strings.Join(created, ","), "first,second,third"; got != want {
		t.Fatalf("created sessions = %q, want %q", got, want)
	}
}
