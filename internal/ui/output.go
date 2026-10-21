package ui

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	OutputTable = "table"
	OutputJSON  = "json"
)

// AddOutputFlag registers the --output/-o flag on a command.
func AddOutputFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("output", "o", OutputTable, "Output format: table, json")
}

// ParseOutputFormat reads and validates the --output flag.
func ParseOutputFormat(cmd *cobra.Command) (string, error) {
	format, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", NewInternalError(fmt.Errorf("failed to read output flag: %w", err))
	}

	if format != OutputTable && format != OutputJSON {
		return "", NewValidationError(fmt.Errorf("invalid output format: %s (supported: table, json)", format))
	}

	return format, nil
}

// PrintJSON writes v to stdout as indented JSON.
func PrintJSON(v any) error {
	jsonBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return NewInternalError(fmt.Errorf("failed to marshal JSON: %w", err))
	}

	fmt.Println(string(jsonBytes))
	return nil
}
