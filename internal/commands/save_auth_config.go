package commands

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/golang-jwt/jwt/v5"
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

	// Parse arguments based on how many were provided
	switch len(args) {
	case 1:
		// JWT mode - extract project_id from token
		projectID, err = extractProjectIDFromJWT(accessToken)
		if err != nil {
			return ui.NewValidationError(fmt.Errorf("failed to extract project_id from JWT: %w", err))
		}
	case 2:
		// Two args: access_token + refresh_token (still extract project_id from JWT)
		refreshToken = args[1]
		projectID, err = extractProjectIDFromJWT(accessToken)
		if err != nil {
			return ui.NewValidationError(fmt.Errorf("failed to extract project_id from JWT: %w", err))
		}
	case 3:
		// Classic mode - all three provided
		refreshToken = args[1]
		projectID = args[2]
	}

	// Flag overrides positional argument and JWT extraction
	if projectIDFlag != "" {
		projectID = projectIDFlag
	}

	// Validate project ID
	if projectID == "" {
		return ui.NewValidationError(fmt.Errorf("project_id is required. Provide it as an argument, via --project-id flag, or use a JWT with project_id claim"))
	}

	if !config.IsValidProjectID(projectID) {
		return ui.NewValidationError(fmt.Errorf("invalid project ID: %s. Project ID should start with 'p-'", projectID))
	}

	// Update config
	cfg.AccessToken = accessToken
	cfg.RefreshToken = refreshToken
	cfg.ProjectID = projectID

	// Save config
	if err := config.Save(cfg); err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to save config: %w", err))
	}

	fmt.Printf("✓ Authentication credentials saved successfully\n")
	fmt.Printf("  Project ID: %s\n", projectID)
	if refreshToken != "" {
		fmt.Printf("  Refresh token: saved\n")
	}

	return nil
}

// extractProjectIDFromJWT extracts the project_id claim from a JWT token
func extractProjectIDFromJWT(tokenString string) (string, error) {
	// Parse without verification - we just need to extract claims
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("invalid JWT token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("failed to parse JWT claims")
	}

	// Try to get project_id from claims
	projectID, ok := claims["project_id"].(string)
	if !ok || projectID == "" {
		return "", fmt.Errorf("JWT token does not contain a valid project_id claim")
	}

	return projectID, nil
}
