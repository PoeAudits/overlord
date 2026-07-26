package tmux

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PoeAudits/overlord/internal/checkpoint"
)

// CaptureCheckpoint records tmux topology only; it never reads pane contents or processes.
func CaptureCheckpoint(runner CommandRunner) (*checkpoint.Checkpoint, error) {
	if os.Getenv("TMUX") == "" {
		return nil, ErrNotInSession
	}
	if runner == nil {
		runner = defaultRunner
	}
	current, err := runner("tmux", "display-message", "-p", "#S")
	if err != nil {
		return nil, fmt.Errorf("determine current session: %w", err)
	}
	result := &checkpoint.Checkpoint{SavedAt: time.Now().UTC(), AttachSession: strings.TrimSpace(string(current))}
	data, err := runner("tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	for order, name := range lines(data) {
		if name == "StartServer" {
			continue
		}
		session, err := captureSession(runner, name, order)
		if err != nil {
			return nil, err
		}
		result.Sessions = append(result.Sessions, session)
	}
	return result, nil
}

func captureSession(runner CommandRunner, name string, order int) (checkpoint.Session, error) {
	session := checkpoint.Session{Name: name, Index: order}
	data, err := runner("tmux", "list-windows", "-t", name, "-F", "#{window_index}\t#{window_name}\t#{window_active}\t#{window_layout}")
	if err != nil {
		return session, fmt.Errorf("list windows for %q: %w", name, err)
	}
	for _, line := range lines(data) {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return session, fmt.Errorf("unexpected window data for %q", name)
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil {
			return session, err
		}
		window := checkpoint.Window{Name: fields[1], Index: index, Layout: fields[3]}
		if fields[2] == "1" {
			session.ActiveWindow = index
		}
		panes, err := runner("tmux", "list-panes", "-t", name+":"+fields[0], "-F", "#{pane_index}\t#{pane_active}\t#{pane_current_path}")
		if err != nil {
			return session, fmt.Errorf("list panes for %q: %w", name, err)
		}
		for _, paneLine := range lines(panes) {
			paneFields := strings.Split(paneLine, "\t")
			if len(paneFields) != 3 {
				return session, fmt.Errorf("unexpected pane data for %q", name)
			}
			paneIndex, err := strconv.Atoi(paneFields[0])
			if err != nil {
				return session, err
			}
			window.Panes = append(window.Panes, checkpoint.Pane{Index: paneIndex, Path: paneFields[2]})
			if paneFields[1] == "1" {
				window.ActivePane = paneIndex
			}
		}
		session.Windows = append(session.Windows, window)
	}
	return session, nil
}

// RestoreCheckpoint creates only missing sessions. Individual failures are reported and do not stop later work.
func RestoreCheckpoint(cp *checkpoint.Checkpoint, runner CommandRunner, warn func(string, ...any)) []string {
	if cp == nil {
		return nil
	}
	if runner == nil {
		runner = defaultRunner
	}
	if warn == nil {
		warn = func(string, ...any) {}
	}
	sessions := append([]checkpoint.Session(nil), cp.Sessions...)
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].Index < sessions[j].Index
	})
	available := make([]string, 0, len(sessions))
	for _, session := range sessions {
		_, err := runner("tmux", "has-session", "-t", session.Name)
		if err == nil {
			available = append(available, session.Name)
			continue
		}
		if code, ok := exitCode(err); !ok || code != 1 {
			warn("failed to check session %q: %v", session.Name, err)
			continue
		}
		if err := restoreSession(session, runner, warn); err != nil {
			warn("failed to restore session %q: %v", session.Name, err)
			continue
		}
		available = append(available, session.Name)
	}
	return available
}

func restoreSession(session checkpoint.Session, runner CommandRunner, warn func(string, ...any)) error {
	if len(session.Windows) == 0 {
		return fmt.Errorf("has no windows")
	}
	first := session.Windows[0]
	if _, err := runner("tmux", "new-session", "-d", "-s", session.Name, "-n", first.Name, "-c", usableDir(first.Panes, warn)); err != nil {
		return err
	}
	moveWindowToIndex(session.Name, first, runner, warn)
	if err := restoreWindow(session.Name, first, false, runner, warn); err != nil {
		warn("failed to restore window %q: %v", first.Name, err)
	}
	for _, window := range session.Windows[1:] {
		if _, err := runner("tmux", "new-window", "-d", "-t", session.Name, "-n", window.Name, "-c", usableDir(window.Panes, warn)); err != nil {
			warn("failed to create window %q: %v", window.Name, err)
			continue
		}
		moveWindowToIndex(session.Name, window, runner, warn)
		if err := restoreWindow(session.Name, window, true, runner, warn); err != nil {
			warn("failed to restore window %q: %v", window.Name, err)
		}
	}
	_, _ = runner("tmux", "select-window", "-t", fmt.Sprintf("%s:%d", session.Name, session.ActiveWindow))
	return nil
}

func moveWindowToIndex(session string, window checkpoint.Window, runner CommandRunner, warn func(string, ...any)) {
	data, err := runner("tmux", "list-windows", "-t", session+":"+window.Name, "-F", "#{window_index}")
	if err != nil {
		warn("failed to inspect window %q index: %v", window.Name, err)
		return
	}
	index, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || index == window.Index {
		return
	}
	if _, err := runner("tmux", "move-window", "-d", "-s", session+":"+window.Name, "-t", fmt.Sprintf("%s:%d", session, window.Index)); err != nil {
		warn("failed to restore index for window %q: %v", window.Name, err)
	}
}

func restoreWindow(session string, window checkpoint.Window, hasInitialPane bool, runner CommandRunner, warn func(string, ...any)) error {
	if len(window.Panes) == 0 {
		return nil
	}
	target := session + ":" + window.Name
	start := 1
	if hasInitialPane {
		start = 1
	}
	for _, pane := range window.Panes[start:] {
		if _, err := runner("tmux", "split-window", "-d", "-t", target, "-c", usableDir([]checkpoint.Pane{pane}, warn)); err != nil {
			warn("failed to create pane in %q: %v", window.Name, err)
		}
	}
	if window.Layout != "" {
		if _, err := runner("tmux", "select-layout", "-t", target, window.Layout); err != nil {
			warn("failed to restore layout for %q: %v", window.Name, err)
		}
	}
	_, _ = runner("tmux", "select-pane", "-t", fmt.Sprintf("%s.%d", target, window.ActivePane))
	return nil
}

func usableDir(panes []checkpoint.Pane, warn func(string, ...any)) string {
	dir, _ := os.UserHomeDir()
	if len(panes) == 0 || panes[0].Path == "" {
		return dir
	}
	if info, err := os.Stat(panes[0].Path); err == nil && info.IsDir() {
		return panes[0].Path
	}
	warn("directory %q is unavailable; using %q", panes[0].Path, dir)
	return dir
}

func lines(data []byte) []string {
	return strings.FieldsFunc(strings.TrimSpace(string(data)), func(r rune) bool { return r == '\n' || r == '\r' })
}
