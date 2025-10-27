package apps

import (
	"context"
	"fmt"
	"os"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type DeleteConfig struct {
	ui.DisplayConfig

	Client    api.Client
	ProjectID string
	AppID     string
}

// DeleteState represents the status of the delete operation
type DeleteState int

const (
	DeleteStateAppDeleting DeleteState = iota
	DeleteStateAppDeleted
	DeleteStateAppDeleteError
)

// DeleteView is the Bubbletea model for deleting an app
type DeleteView struct {
	ctx context.Context

	// State
	status  DeleteState
	spinner *ui.SpinnerModel
	err     error

	conf DeleteConfig
}

// NewDeleteView creates a new app delete view
func NewDeleteView(ctx context.Context, conf DeleteConfig) *DeleteView {
	return &DeleteView{
		ctx:     ctx,
		status:  DeleteStateAppDeleting,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Error returns the error if any occurred during execution
func (m *DeleteView) Error() error {
	return m.err
}

// Init starts the deletion
func (m *DeleteView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.deleteApp)
}

// Update handles messages
func (m *DeleteView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal
		if m.conf.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nCancelled\n")
		}
		return m, tea.Quit

	case appDeletedMsg:
		m.status = DeleteStateAppDeleted

		if m.conf.SimpleOutput() {
			fmt.Printf("App deleted successfully.\n")
		}

		return m, tea.Quit

	case *ui.UIError:
		msg.SilentExit = true
		m.err = msg
		m.status = DeleteStateAppDeleteError

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

	case tea.KeyMsg:
		// Only handle keyboard input in interactive mode
		if !m.conf.SimpleOutput() && msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	default:
		// Update spinner only in interactive mode
		if !m.conf.SimpleOutput() && m.status == DeleteStateAppDeleting {
			var cmd tea.Cmd
			spinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the output
func (m *DeleteView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	switch m.status {
	case DeleteStateAppDeleting:
		return fmt.Sprintf("%s Deleting app '%s'...", m.spinner.View(), m.conf.AppID)

	case DeleteStateAppDeleted:
		return ui.SuccessStyle.Render(fmt.Sprintf("✓ App '%s' deleted successfully\n", m.conf.AppID))

	case DeleteStateAppDeleteError:
		if m.err != nil {
			return ui.FormatError(m.err)
		}
		return ui.ErrorStyle.Render("✗ Failed to delete app\n")

	default:
		return ""
	}
}

// Messages

type appDeletedMsg struct{}

// Commands (async operations)

func (m *DeleteView) deleteApp() tea.Msg {
	err := m.conf.Client.DeleteApp(m.ctx, m.conf.ProjectID, m.conf.AppID)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return appDeletedMsg{}
}
