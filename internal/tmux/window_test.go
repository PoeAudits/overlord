package tmux

import (
	"reflect"
	"testing"
)

func TestOpenWindowInCurrentSession_SelectsExistingWindow(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	var calls [][]string
	runner := func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch args[0] {
		case "display-message":
			return []byte("work\n"), nil
		case "list-windows":
			return []byte("shell\ncontainer\n"), nil
		default:
			return nil, nil
		}
	}

	if err := OpenWindowInCurrentSession("container", "/repo/container", runner); err != nil {
		t.Fatalf("OpenWindowInCurrentSession() returned error: %v", err)
	}
	want := [][]string{
		{"tmux", "display-message", "-p", "#S"},
		{"tmux", "list-windows", "-t", "work", "-F", "#{window_name}"},
		{"tmux", "select-window", "-t", "work:container"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestOpenWindowInCurrentSession_CreatesProjectWindow(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	var calls [][]string
	runner := func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		switch args[0] {
		case "display-message":
			return []byte("work\n"), nil
		case "list-windows":
			return []byte("shell\n"), nil
		default:
			return nil, nil
		}
	}

	if err := OpenWindowInCurrentSession("container", "/repo/container", runner); err != nil {
		t.Fatalf("OpenWindowInCurrentSession() returned error: %v", err)
	}
	want := [][]string{
		{"tmux", "display-message", "-p", "#S"},
		{"tmux", "list-windows", "-t", "work", "-F", "#{window_name}"},
		{"tmux", "new-window", "-t", "work", "-n", "container", "-c", "/repo/container"},
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
}

func TestOpenWindowInCurrentSession_RejectsNonTmuxShell(t *testing.T) {
	t.Setenv("TMUX", "")
	if err := OpenWindowInCurrentSession("container", "/repo/container", nil); err != ErrNotInSession {
		t.Fatalf("error = %v, want %v", err, ErrNotInSession)
	}
}
