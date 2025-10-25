package commands

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cerebriumai/cerebrium/internal/statuspage"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiCommands "github.com/cerebriumai/cerebrium/internal/ui/commands"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// NewStatusCmd creates a status command
func NewStatusCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check Cerebrium service status",
		Long: `Check the operational status of Cerebrium services.

This command queries the Cerebrium status page to show you whether all services
are operational or if there are any ongoing incidents affecting the platform.

Example:
  cerebrium status
  cerebrium status --output json    # Output as JSON for automation
  cerebrium status --no-color       # Disable animations and colors`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(cmd, outputFormat)
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json")

	return cmd
}

func runStatus(cmd *cobra.Command, outputFormat string) error {
	cmd.SilenceUsage = true

	// Validate output format
	if outputFormat != "table" && outputFormat != "json" {
		return ui.NewValidationError(fmt.Errorf("invalid output format: %s (supported: table, json)", outputFormat))
	}

	// For JSON output, bypass the UI and fetch data directly
	if outputFormat == "json" {
		return runStatusJSON(cmd)
	}

	// Regular table output - use existing UI flow
	// Get display options from context (loaded once in root command)
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get display options: %w", err))
	}

	// Create statuspage client
	client := statuspage.NewClient(nil) // nil uses default HTTP client

	// Create Bubbletea model for status display
	model := uiCommands.NewStatusView(cmd.Context(), uiCommands.StatusConfig{
		DisplayConfig: displayOpts,
		Client:        client,
	})

	// Configure Bubbletea based on display options
	var programOpts []tea.ProgramOption

	if !displayOpts.IsInteractive {
		// Non-TTY mode or animation disabled: disable renderer and input
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	}

	// Run Bubbletea program
	p := tea.NewProgram(model, programOpts...)

	// Set up signal handling for graceful cancellation
	doneCh := ui.SetupSignalHandling(p, 0)
	defer close(doneCh)

	finalModel, err := p.Run()
	if err != nil {
		return ui.NewInternalError(fmt.Errorf("internal error: %w", err))
	}

	// Extract model and check for errors
	m, ok := finalModel.(*uiCommands.StatusView)
	if !ok {
		return ui.NewInternalError(fmt.Errorf("unexpected model type"))
	}

	// Handle error from model
	if uiErr := m.GetError(); uiErr != nil {
		if uiErr.SilentExit {
			// Error was already shown in UI or should be silent - exit cleanly
			return nil
		}
		// Return error for Cobra/main.go to print
		return uiErr
	}

	return nil
}

// JSONComponent output structures
type JSONComponent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type JSONIncident struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Status             string          `json:"status"`
	URL                string          `json:"url"`
	LastUpdateMessage  string          `json:"last_update_message,omitempty"`
	CurrentWorstImpact string          `json:"current_worst_impact"`
	AffectedComponents []JSONComponent `json:"affected_components"`
}

type JSONStatusOutput struct {
	Timestamp        time.Time       `json:"timestamp"`
	OverallStatus    string          `json:"overall_status"` // "operational" or "incidents"
	OngoingIncidents []JSONIncident  `json:"ongoing_incidents"`
	Components       []JSONComponent `json:"components"`
}

// runStatusJSON handles JSON output format
func runStatusJSON(cmd *cobra.Command) error {
	// Create statuspage client
	client := statuspage.NewClient(nil)

	// Fetch status directly
	status, err := client.GetStatus(cmd.Context())
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to fetch status: %w", err))
	}

	// Convert to JSON structure
	output := JSONStatusOutput{
		Timestamp:        time.Now().UTC(),
		OverallStatus:    "operational",
		OngoingIncidents: make([]JSONIncident, 0),
		Components:       make([]JSONComponent, 0),
	}

	// Add components (sorted by name)
	components := make([]statuspage.Component, len(status.AllComponents))
	copy(components, status.AllComponents)
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	for _, comp := range components {
		output.Components = append(output.Components, JSONComponent{
			ID:     comp.ID,
			Name:   comp.Name,
			Status: string(comp.Status),
		})
	}

	// Add incidents if any
	if len(status.OngoingIncidents) > 0 {
		output.OverallStatus = "incidents"

		for _, incident := range status.OngoingIncidents {
			jsonIncident := JSONIncident{
				ID:                 incident.ID,
				Name:               incident.Name,
				Status:             string(incident.Status),
				URL:                incident.URL,
				LastUpdateMessage:  incident.LastUpdateMessage,
				CurrentWorstImpact: incident.CurrentWorstImpact,
				AffectedComponents: make([]JSONComponent, 0),
			}

			// Add affected components
			for _, comp := range incident.AffectedComponents {
				jsonIncident.AffectedComponents = append(jsonIncident.AffectedComponents, JSONComponent{
					ID:     comp.ID,
					Name:   comp.Name,
					Status: string(comp.Status),
				})
			}

			output.OngoingIncidents = append(output.OngoingIncidents, jsonIncident)
		}
	}

	// Output JSON
	jsonBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return ui.NewInternalError(fmt.Errorf("failed to marshal JSON: %w", err))
	}

	fmt.Println(string(jsonBytes))
	return nil
}
