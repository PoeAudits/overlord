package cmd

import "github.com/charmbracelet/lipgloss"

// Color constants for the shared palette.
// Use these when building custom one-off styles in specific commands.
var (
	colorCyan      = lipgloss.Color("86")
	colorGreen     = lipgloss.Color("82")
	colorYellow    = lipgloss.Color("220")
	colorRed       = lipgloss.Color("196")
	colorGray      = lipgloss.Color("245")
	colorPurple    = lipgloss.Color("99")
	colorWhite     = lipgloss.Color("255")
	colorOrange    = lipgloss.Color("214")
	colorBlue      = lipgloss.Color("75")
	colorLightBlue = lipgloss.Color("39")
	colorMagenta   = lipgloss.Color("201")
	colorLime      = lipgloss.Color("156")
	colorAmber     = lipgloss.Color("208")
	colorTeal      = lipgloss.Color("37")
	colorLavender  = lipgloss.Color("141")
	colorPink      = lipgloss.Color("213")
)

// styles holds the common lipgloss styles used across commands.
type styles struct {
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
	Lang     lipgloss.Style

	// TUI picker styles
	Prompt   lipgloss.Style
	Selected lipgloss.Style
	Name     lipgloss.Style
	Desc     lipgloss.Style
	Cursor   lipgloss.Style

	// Specialty styles
	StrongWarn  lipgloss.Style
	Destructive lipgloss.Style

	// List table styles
	TableHeader        lipgloss.Style
	TableCell          lipgloss.Style
	TableName          lipgloss.Style
	TableMuted         lipgloss.Style
	TableRecent        lipgloss.Style
	TableStatusSynced  lipgloss.Style
	TableStatusActive  lipgloss.Style
	TableStatusDefault lipgloss.Style
}

// NewStyles returns a styles instance with the standard color palette.
func NewStyles() styles {
	return styles{
		// Universal output styles
		Label:   lipgloss.NewStyle().Foreground(colorCyan).Bold(true),
		Success: lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(colorYellow),
		Error:   lipgloss.NewStyle().Foreground(colorRed).Bold(true),
		Info:    lipgloss.NewStyle().Foreground(colorGray),
		Path:    lipgloss.NewStyle().Foreground(colorGray),
		Bold:    lipgloss.NewStyle().Foreground(colorWhite).Bold(true),
		Dim:     lipgloss.NewStyle().Foreground(colorGray),

		// Info display styles
		Section:  lipgloss.NewStyle().Foreground(colorCyan).Bold(true),
		Value:    lipgloss.NewStyle().Foreground(colorWhite),
		Category: lipgloss.NewStyle().Foreground(colorPurple),
		Lang:     lipgloss.NewStyle().Foreground(colorYellow),

		// TUI picker styles
		Prompt:   lipgloss.NewStyle().Foreground(colorCyan).Bold(true),
		Selected: lipgloss.NewStyle().Foreground(colorGreen).Bold(true),
		Name:     lipgloss.NewStyle().Foreground(colorWhite),
		Desc:     lipgloss.NewStyle().Foreground(colorGray),
		Cursor:   lipgloss.NewStyle().Foreground(colorGreen).Bold(true),

		// Specialty styles
		StrongWarn:  lipgloss.NewStyle().Foreground(colorOrange).Bold(true),
		Destructive: lipgloss.NewStyle().Foreground(colorRed).Bold(true),

		// List table styles
		TableHeader:        lipgloss.NewStyle().Bold(true).Foreground(colorCyan).Padding(0, 1),
		TableCell:          lipgloss.NewStyle().Padding(0, 1),
		TableName:          lipgloss.NewStyle().Foreground(colorWhite).Bold(true).Padding(0, 1),
		TableMuted:         lipgloss.NewStyle().Foreground(colorGray).Padding(0, 1),
		TableRecent:        lipgloss.NewStyle().Foreground(colorCyan).Padding(0, 1),
		TableStatusSynced:  lipgloss.NewStyle().Foreground(colorPink).Padding(0, 1),
		TableStatusActive:  lipgloss.NewStyle().Foreground(colorGreen).Padding(0, 1),
		TableStatusDefault: lipgloss.NewStyle().Foreground(colorGray).Padding(0, 1),
	}
}
