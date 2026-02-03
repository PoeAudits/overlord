package cmd

import "github.com/charmbracelet/lipgloss"

// Color constants for the shared palette.
// Use these when building custom one-off styles in specific commands.
var (
	ColorCyan   = lipgloss.Color("86")
	ColorGreen  = lipgloss.Color("82")
	ColorYellow = lipgloss.Color("220")
	ColorRed    = lipgloss.Color("196")
	ColorGray   = lipgloss.Color("245")
	ColorPurple = lipgloss.Color("99")
	ColorWhite  = lipgloss.Color("255")
)

// Styles holds the common lipgloss styles used across commands.
type Styles struct {
	// Universal output styles
	Label   lipgloss.Style
	Success lipgloss.Style
	Warn    lipgloss.Style
	Error   lipgloss.Style
	Info    lipgloss.Style
	Path    lipgloss.Style
	Bold    lipgloss.Style
	Dim     lipgloss.Style

	// Info display styles
	Section  lipgloss.Style
	Value    lipgloss.Style
	Category lipgloss.Style

	// TUI picker styles
	Prompt   lipgloss.Style
	Selected lipgloss.Style
	Name     lipgloss.Style
	Desc     lipgloss.Style
	Cursor   lipgloss.Style

	// Specialty styles
	StrongWarn lipgloss.Style
}

// NewStyles returns a Styles instance with the standard color palette.
func NewStyles() Styles {
	return Styles{
		// Universal output styles
		Label:   lipgloss.NewStyle().Foreground(ColorCyan).Bold(true),
		Success: lipgloss.NewStyle().Foreground(ColorGreen).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(ColorYellow),
		Error:   lipgloss.NewStyle().Foreground(ColorRed).Bold(true),
		Info:    lipgloss.NewStyle().Foreground(ColorGray),
		Path:    lipgloss.NewStyle().Foreground(ColorGray),
		Bold:    lipgloss.NewStyle().Foreground(ColorWhite).Bold(true),
		Dim:     lipgloss.NewStyle().Foreground(ColorGray),

		// Info display styles
		Section:  lipgloss.NewStyle().Foreground(ColorCyan).Bold(true),
		Value:    lipgloss.NewStyle().Foreground(ColorWhite),
		Category: lipgloss.NewStyle().Foreground(ColorPurple),

		// TUI picker styles
		Prompt:   lipgloss.NewStyle().Foreground(ColorCyan).Bold(true),
		Selected: lipgloss.NewStyle().Foreground(ColorGreen).Bold(true),
		Name:     lipgloss.NewStyle().Foreground(ColorWhite),
		Desc:     lipgloss.NewStyle().Foreground(ColorGray),
		Cursor:   lipgloss.NewStyle().Foreground(ColorGreen).Bold(true),

		// Specialty styles
		StrongWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true),
	}
}
