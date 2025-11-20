package ui

import (
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// RenderPanel creates a bordered panel with a title (similar to Python's rich Panel.fit)
// The title appears in the top border line
func RenderPanel(title, content string) string {
	// For titled panels, we need to construct the border manually
	// because lipgloss doesn't support multi-character border elements

	// First, render the content with padding
	paddingStyle := lipgloss.NewStyle().Padding(1, 2, 0) // top: 1, sides: 2, bottom: 0
	paddedContent := paddingStyle.Render(content)

	// Split content into lines to get dimensions
	lines := strings.Split(paddedContent, "\n")

	// Find the maximum width
	maxWidth := 0
	for _, line := range lines {
		width := lipgloss.Width(line)
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Pad all lines to the same width
	for i, line := range lines {
		currentWidth := lipgloss.Width(line)
		if currentWidth < maxWidth {
			lines[i] = line + strings.Repeat(" ", maxWidth-currentWidth)
		}
	}

	// Create border color style
	borderColor := lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// Create title with styling
	styledTitle := " " + lipgloss.NewStyle().
		Foreground(lipgloss.Color("11")).
		Bold(true).
		Render(title) + " "

	titleWidth := lipgloss.Width(styledTitle)

	// Construct top border with title
	topPadding := 3                                          // "╭──"
	remainingWidth := maxWidth + 2 - titleWidth - topPadding // +2 for the left and right borders
	if remainingWidth < 1 {
		remainingWidth = 1
	}

	topLine := borderColor.Render("╭─") +
		styledTitle +
		borderColor.Render(strings.Repeat("─", remainingWidth)+"╮")

	// Build the complete bordered box
	var result strings.Builder
	result.WriteString(topLine + "\n")

	// Add middle lines with borders
	for _, line := range lines {
		result.WriteString(borderColor.Render("│") + line + borderColor.Render("│") + "\n")
	}

	// Add bottom border
	bottomLine := borderColor.Render("╰" + strings.Repeat("─", maxWidth) + "╯\n")
	result.WriteString(bottomLine)

	return result.String()
}

// RenderDetailTable renders a two-column table with sections
// This is used for app details, project details, etc.
func RenderDetailTable(sections []TableSection) string {
	var output strings.Builder

	labelStyle := lipgloss.NewStyle().Bold(true).Width(30)
	sectionHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Underline(true)

	for i, section := range sections {
		if section.Header != "" {
			if i > 0 {
				output.WriteString("\n")
			}

			// Section header
			output.WriteString(sectionHeaderStyle.Render(section.Header))
			output.WriteString("\n\n")
		}

		// Section rows
		for _, row := range section.Rows {
			output.WriteString(labelStyle.Render(row.Label))
			output.WriteString(row.Value)
			output.WriteString("\n")
		}
	}

	return output.String()
}

// TableSection represents a section in a detail table
type TableSection struct {
	Header string
	Rows   []TableRow
}

// TableRow represents a row in a detail table
type TableRow struct {
	Label string
	Value string
}
