package tui

import "github.com/charmbracelet/lipgloss"

// Colors from the lectern logo. AdaptiveColor picks the Light or Dark value
// based on the terminal's background.
var (
	colorOrange = lipgloss.Color("#E0892B")
	colorText   = lipgloss.AdaptiveColor{Light: "#1B1C1F", Dark: "#F6F5F1"}
	colorMuted  = lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"}
	colorRule   = lipgloss.AdaptiveColor{Light: "#D9D6CF", Dark: "#3A3A3A"}
)

var (
	titleStyle  = lipgloss.NewStyle().Foreground(colorText).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	accentStyle = lipgloss.NewStyle().Foreground(colorOrange)
	errorStyle  = lipgloss.NewStyle().Foreground(colorOrange)
	keyStyle    = lipgloss.NewStyle().Foreground(colorOrange)
)
