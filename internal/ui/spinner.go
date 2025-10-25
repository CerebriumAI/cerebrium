package ui

import (
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// SpinnerModel provides a reusable spinner component
type SpinnerModel struct {
	spinner spinner.Model
}

// NewSpinner creates a new spinner
func NewSpinner() *SpinnerModel {
	return &SpinnerModel{spinner: spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(SpinnerStyle),
	)}
}

// Init returns the initial spinner tick command
func (m *SpinnerModel) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles spinner tick messages
func (m *SpinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// View returns the current spinner frame
func (m *SpinnerModel) View() string {
	return m.spinner.View()
}
