// Package ui contains shared lipgloss styles used across CLI/TUI surfaces.
package ui

import "github.com/charmbracelet/lipgloss"

// Palette is the canonical color set for flyflor. Inspired by Charm's own
// brand: violet/pink primaries on a dark slate background, kept WCAG-friendly
// on common dark terminal themes.
var (
	ColorBg      = lipgloss.Color("#0F0B1A")
	ColorSurface = lipgloss.Color("#1A1226")
	ColorBorder  = lipgloss.Color("#3F2C5C")
	ColorMuted   = lipgloss.Color("#8A7A99")
	ColorInk     = lipgloss.Color("#F5EFFF")
	ColorPrimary = lipgloss.Color("#C77DFF")
	ColorAccent  = lipgloss.Color("#FF7FD8")
	ColorOk      = lipgloss.Color("#7AE0B2")
	ColorWarn    = lipgloss.Color("#F6C177")
	ColorErr     = lipgloss.Color("#FF6B81")
)

// Styles bundles the most-used lipgloss styles so callers don't re-declare.
var (
	Title = lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	Subtitle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	Brand = lipgloss.NewStyle().
		Foreground(ColorBg).
		Background(ColorPrimary).
		Bold(true).
		Padding(0, 1)

	Chip = lipgloss.NewStyle().
		Foreground(ColorInk).
		Background(ColorSurface).
		Padding(0, 1).
		MarginRight(1)

	ChipOk    = Chip.Foreground(ColorOk)
	ChipWarn  = Chip.Foreground(ColorWarn)
	ChipError = Chip.Foreground(ColorErr)

	UserHeader = lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

	AssistantHeader = lipgloss.NewStyle().
			Foreground(ColorAccent).
			Bold(true)

	UserBubble = lipgloss.NewStyle().
			Foreground(ColorInk).
			Background(lipgloss.Color("#26203A")).
			Padding(0, 1).
			MarginTop(1)

	Hint = lipgloss.NewStyle().
		Foreground(ColorMuted).
		Italic(true)

	Error = lipgloss.NewStyle().
		Foreground(ColorErr).
		Bold(true)

	Border = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)
)
