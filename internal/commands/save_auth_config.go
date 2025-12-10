package commands

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

// NewSaveAuthConfigCmd creates the save-auth-config command
func NewSaveAuthConfigCmd() *cobra.Command {
	var projectID string

	cmd := &cobra.Command{
		Use:   "save-auth-config ACCESS_TOKEN [REFRESH_TOKEN] [PROJECT_ID]",
		Short: "Save authentication credentials to config file",
		Long: `Saves the access token, refresh token, and project ID to the config file.
This function is a helper method to allow users to store credentials
directly for the framework. Mostly used for CI/CD.

Supports two modes:
1. JWT mode: Provide only a JWT token, project_id will be extracted automatically
2. Classic mode: Provide access_token, refresh_token, and project_id

Examples:
  # JWT mode - extract project_id from token
  cerebrium save-auth-config <jwt-token>

  # Classic mode - provide all credentials
  cerebrium save-auth-config <access-token> <refresh-token> <project-id>

  # Override project_id with flag
  cerebrium save-auth-config <jwt-token> --project-id p-xxxxx`,
		Args: cobra.RangeArgs(1, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSaveAuthConfig(cmd, args, projectID)
		},
	}

	cmd.Flags().StringVar(&projectID, "project-id", "", "Project ID to save (overrides JWT extraction and positional argument)")

	return cmd
}

func runSaveAuthConfig(cmd *cobra.Command, args []string, projectIDFlag string) error {
	cmd.SilenceUsage = true

	// Get config from context
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	accessToken := args[0]
	var refreshToken string
	var projectID string

	// Parse arguments
	if len(args) >= 2 {
		refreshToken = args[1]
	}
	if len(args) >= 3 {
		projectID = args[2]
	}

	// Flag overrides positional argument
	if projectIDFlag != "" {
		projectID = projectIDFlag
	}

	// Determine mode based on Python's logic:
	// If refresh_token AND project_id are provided → classic mode
	// Otherwise → JWT mode (service account token)
	isClassicMode := refreshToken != "" && projectID != ""

	if isClassicMode {
		// Classic mode: save to accessToken + refreshToken
		if !config.IsValidProjectID(projectID) {
			return ui.NewValidationError(fmt.Errorf("invalid project ID: %s. Project ID should start with 'p-'", projectID))
		}

		cfg.AccessToken = accessToken
		cfg.RefreshToken = refreshToken
		cfg.ProjectID = projectID
		// Clear service account token when using classic mode
		cfg.ServiceAccountToken = ""

		if err := config.Save(cfg); err != nil {
			return ui.NewFileSystemError(fmt.Errorf("failed to save config: %w", err))
		}

		fmt.Printf("✓ Authentication credentials saved successfully\n")
		fmt.Printf("  Project ID: %s\n", projectID)
		fmt.Printf("  Refresh token: saved\n")
	} else {
		// JWT mode: save to serviceAccountToken, extract project_id from JWT
		claims, err := auth.ParseClaims(accessToken)
		if err != nil {
			return ui.NewValidationError(fmt.Errorf("failed to parse JWT: %w", err))
		}

		// Use extracted project_id, but allow override via flag
		if projectID == "" {
			projectID = config.ExtractProjectIDFromClaims(claims)
			if projectID == "" {
				return ui.NewValidationError(fmt.Errorf("JWT token does not contain a valid project_id claim"))
			}
		}

		if !config.IsValidProjectID(projectID) {
			return ui.NewValidationError(fmt.Errorf("invalid project ID: %s. Project ID should start with 'p-'", projectID))
		}

		cfg.ServiceAccountToken = accessToken
		cfg.ProjectID = projectID
		// Clear regular tokens when using service account
		cfg.AccessToken = ""
		cfg.RefreshToken = ""

		if err := config.Save(cfg); err != nil {
			return ui.NewFileSystemError(fmt.Errorf("failed to save config: %w", err))
		}

		fmt.Printf("✓ Service account token saved successfully\n")
		fmt.Printf("  Project ID: %s\n", projectID)
	}

	return nil
}
