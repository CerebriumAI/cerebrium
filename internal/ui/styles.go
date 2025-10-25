package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Status colors
	GreenStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	RedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	YellowStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	CyanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	MagentaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	BoldStyle    = lipgloss.NewStyle().Bold(true)

	// Progress states
	SuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	ActiveStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	PendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	URLStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	// UI elements (using magenta for spinner to match Python yaspin color)
	SpinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	ErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// Table styling
	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true).
			Padding(0, 1)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("11")).
			Padding(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Underline(true)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Padding(0, 1)

	// Log timestamp - subtle gray to distinguish from log content
	TimestampStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))
)
