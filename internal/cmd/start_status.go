package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"

	"github.com/PoeAudits/overlord/internal/fileutil"
	"github.com/PoeAudits/overlord/internal/stack"
	"github.com/PoeAudits/overlord/internal/tmux"
)

var (
	startStatusLoadStackConfig       = loadStackConfig
	startStatusSessionExists         = func() (bool, error) { return tmux.NewSession(stack.SessionName, "").Exists() }
	startStatusDefaultServices       = stack.DefaultServices
	startStatusRemoteT3CodeURL       = stack.RemoteT3CodeURL
	startStatusResolveOpenCodeBinary = stack.ResolveOpenCodeBinary
	startStatusInspectOCShim         = inspectOCShim
)

var startStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show development stack status",
	Args:  cobra.NoArgs,
	RunE:  runStartStatus,
}

func init() {
	startCmd.AddCommand(startStatusCmd)
}

func runStartStatus(cmd *cobra.Command, args []string) error {
	s := NewStyles()
	w := cmd.ErrOrStderr()

	exists, err := startStatusSessionExists()
	if err != nil {
		return cmdError("start status", "failed to check session", err.Error())
	}

	cfg, err := startStatusLoadStackConfig()
	if err != nil {
		return cmdError("start status", "failed to load stack config", err.Error())
	}

	if !exists {
		fmt.Fprintf(w, "  %s\n", s.Warn.Render(stack.SessionName+" is not running"))
		fmt.Fprintln(w)
		printStartStatusEndpointDetails(w, cfg, s)
		return nil
	}

	services := startStatusDefaultServices(cfg)

	fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render(stack.SessionName), s.Success.Render("running"))

	type statusEntry struct {
		name string
		err  error
	}
	entries := make([]statusEntry, len(services))
	var wg sync.WaitGroup
	for i, svc := range services {
		wg.Add(1)
		go func(idx int, sv stack.Service) {
			defer wg.Done()
			entries[idx] = statusEntry{name: sv.WindowName(), err: sv.HealthCheck()}
		}(i, svc)
	}
	wg.Wait()

	for _, e := range entries {
		if e.err == nil {
			fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render(e.name), s.Success.Render("healthy"))
		} else {
			fmt.Fprintf(w, "  %-14s %s  %s\n", s.Label.Render(e.name), s.Warn.Render("unhealthy"), e.err.Error())
		}
	}

	fmt.Fprintln(w)
	printStartStatusEndpointDetails(w, cfg, s)

	return nil
}

func printStartStatusEndpointDetails(w io.Writer, cfg *stack.StackConfig, s styles) {
	fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render("T3 Code URL"), s.Value.Render(stack.T3CodeURL(cfg)))
	fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render("OpenCode URL"), s.Value.Render(stack.OpenCodeURL(cfg)))

	remoteURL, remoteErr := startStatusRemoteT3CodeURL(cfg)
	if remoteErr != nil {
		fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render("Remote URL"), s.Dim.Render("unavailable"))
	} else {
		fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render("Remote URL"), s.Value.Render(remoteURL))
	}

	fmt.Fprintf(w, "  %-14s %s\n", s.Label.Render("OpenCode Bin"), s.Value.Render(formatOpenCodeBinaryDisplay(startStatusResolveOpenCodeBinary())))

	shimState, err := startStatusInspectOCShim()
	if err != nil {
		fmt.Fprintf(w, "  %-14s %s  %s\n", s.Label.Render("oc Shim"), s.Dim.Render("unavailable"), err.Error())
		return
	}

	statusStyle := s.Success
	if shimState.Status != ocShimCurrent {
		statusStyle = s.Warn
	}
	fmt.Fprintf(w, "  %-14s %s  %s\n", s.Label.Render("oc Shim"), s.Value.Render(fileutil.FormatPathWithTilde(shimState.Path)), statusStyle.Render(string(shimState.Status)))
}

func formatOpenCodeBinaryDisplay(path string) string {
	if filepath.IsAbs(path) {
		return fileutil.FormatPathWithTilde(path)
	}
	return path + " (PATH lookup)"
}
