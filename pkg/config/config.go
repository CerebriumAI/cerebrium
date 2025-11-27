package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	DefaultConfigDir  = ".cerebrium"
	DefaultConfigFile = "config.yaml"
)

// Config holds the CLI configuration
type Config struct {
	environment         Environment
	envConfig           *EnvConfig
	ProjectID           string
	AccessToken         string
	RefreshToken        string
	ServiceAccountToken string // JWT token from save-auth-config, takes precedence over AccessToken
	DefaultRegion       string
	SkipVersionCheck    bool
	LogLevel            string
	TelemetryEnabled    *bool // Pointer to distinguish between unset (nil) and explicitly set (true/false)
}

// ValidUserFacingConfigKeys lists config keys that users should interact with
// (excludes tokens which are managed by login, and hides environment prefixes)
var ValidUserFacingConfigKeys = map[string]bool{
	// Global settings
	"skipversioncheck": true,
	"loglevel":         true,
	"telemetry":        true,

	// Environment-specific settings (user doesn't need to know about env prefixes)
	"defaultregion": true,
	"project":       true,
}

// IsValidUserFacingKey checks if a config key is a recognized user-facing key
func IsValidUserFacingKey(key string) bool {
	return ValidUserFacingConfigKeys[key]
}

// GetConfigKeyDescription returns a description for a config key
func GetConfigKeyDescription(key string) string {
	descriptions := map[string]string{
		"skipversioncheck": "Disable automatic version update checks (true/false)",
		"loglevel":         "Logging level (debug/info/warn/error, default: info)",
		"telemetry":        "Enable error telemetry and crash reporting (true/false, default: true)",
		"defaultregion":    "Default region for deployments (e.g., us-east-1, us-west-2)",
		"project":          "Current project ID",
		"accesstoken":      "OAuth access token (managed by 'cerebrium login')",
		"refreshtoken":     "OAuth refresh token (managed by 'cerebrium login')",
	}
	return descriptions[key]
}

// GetEnvironmentPrefixedKey returns the key with environment prefix
// For user-facing commands, users work with unprefixed keys (e.g., "project")
// This function adds the environment prefix (e.g., "dev-project") automatically
func GetEnvironmentPrefixedKey(key string, env Environment) string {
	// Global keys (not environment-specific)
	globalKeys := map[string]bool{
		"skipversioncheck": true,
		"loglevel":         true,
	}

	if globalKeys[key] {
		return key
	}

	// Add environment prefix
	return getKeyPrefix(env) + key
}

// GetUserFacingKeys returns the list of keys users should interact with
// Strips environment prefixes to keep it simple for users
func GetUserFacingKeys() []string {
	return []string{
		"skip-version-check",
		"log-level",
		"default-region",
		"project",
	}
}

// Load reads the configuration from ~/.cerebrium/config.yaml
func Load() (*Config, error) {
	env := GetEnvironment()
	envConfig, err := GetEnvConfig(env)
	if err != nil {
		return nil, fmt.Errorf("failed to get environment config: %w", err)
	}

	// Set up Viper to read from ~/.cerebrium/config.yaml
	configPath := getConfigPath()
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Create config file if it doesn't exist
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := ensureConfigDir(); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
		if err := viper.WriteConfig(); err != nil {
			return nil, fmt.Errorf("failed to create config file: %w", err)
		}
	}

	// Read the config
	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	// Get environment-specific keys
	prefix := getKeyPrefix(env)

	config := &Config{
		environment:         env,
		envConfig:           envConfig,
		ProjectID:           viper.GetString(prefix + "project"),
		AccessToken:         viper.GetString(prefix + "accesstoken"),
		RefreshToken:        viper.GetString(prefix + "refreshtoken"),
		ServiceAccountToken: viper.GetString(prefix + "serviceaccounttoken"),
		DefaultRegion:       viper.GetString(prefix + "defaultregion"),
		SkipVersionCheck:    viper.GetBool("skipversioncheck"), // Global setting (not env-specific)
		LogLevel:            viper.GetString("loglevel"),       // Global setting (not env-specific)
	}

	// Handle telemetry setting - use pointer to distinguish unset from false
	if viper.IsSet("telemetry") {
		telemetryEnabled := viper.GetBool("telemetry")
		config.TelemetryEnabled = &telemetryEnabled
	}

	return config, nil
}

// IsTelemetryEnabled returns whether telemetry is enabled.
// Returns true by default if not explicitly set (opt-out model).
func (c *Config) IsTelemetryEnabled() bool {
	// Check environment variable first (highest priority)
	if envVal := os.Getenv("CEREBRIUM_TELEMETRY_DISABLED"); envVal != "" {
		return envVal != "true" && envVal != "1"
	}

	// Check config file setting
	if c.TelemetryEnabled != nil {
		return *c.TelemetryEnabled
	}

	// Default to enabled (opt-out model)
	return true
}

// Save writes the current configuration to disk
func Save(config *Config) error {
	prefix := getKeyPrefix(config.environment)

	viper.Set(prefix+"project", config.ProjectID)
	viper.Set(prefix+"accesstoken", config.AccessToken)
	viper.Set(prefix+"refreshtoken", config.RefreshToken)
	viper.Set(prefix+"serviceaccounttoken", config.ServiceAccountToken)
	viper.Set(prefix+"defaultregion", config.DefaultRegion)
	viper.Set("skipversioncheck", config.SkipVersionCheck) // Global setting
	viper.Set("loglevel", config.LogLevel)                 // Global setting

	// Save telemetry setting if explicitly set
	if config.TelemetryEnabled != nil {
		viper.Set("telemetry", *config.TelemetryEnabled)
	}

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// getConfigPath returns the full path to the config file
func getConfigPath() string {
	if path := os.Getenv("CEREBRIUM_CONFIG_PATH"); path != "" {
		return path
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback
		return filepath.Join(".", DefaultConfigDir, DefaultConfigFile)
	}

	return filepath.Join(homeDir, DefaultConfigDir, DefaultConfigFile)
}

// Context key for storing config
type contextKey string

const configContextKey contextKey = "config"

// GetConfigFromContext retrieves the config from the command context
func GetConfigFromContext(cmd *cobra.Command) (*Config, error) {
	ctx := cmd.Context()
	if ctx == nil {
		return nil, fmt.Errorf("no context available")
	}

	cfg, ok := ctx.Value(configContextKey).(*Config)
	if !ok || cfg == nil {
		return nil, fmt.Errorf("config not found in context")
	}

	return cfg, nil
}

// GetContextKey returns the context key used for storing config
// This is needed by root.go to store the config in context
func GetContextKey() interface{} {
	return configContextKey
}

// GetCurrentProject returns the current project ID from config
func (c *Config) GetCurrentProject() (string, error) {
	if c.ProjectID == "" {
		return "", fmt.Errorf("no project configured. Please set your project ID")
	}

	if !IsValidProjectID(c.ProjectID) {
		return "", fmt.Errorf("invalid project ID: %s", c.ProjectID)
	}

	return c.ProjectID, nil
}

// SetCurrentProject sets and saves the current project ID
func (c *Config) SetCurrentProject(projectID string) error {
	if !IsValidProjectID(projectID) {
		return fmt.Errorf("invalid project ID: %s. Project ID should start with 'p-'", projectID)
	}

	c.ProjectID = projectID
	return Save(c)
}

// IsValidProjectID checks if a project ID is valid based on the current environment
func IsValidProjectID(projectID string) bool {
	env := GetEnvironment()

	// Production environment: only accept p- prefix
	if env == EnvProd {
		return strings.HasPrefix(projectID, "p-")
	}

	// Dev/local environments: accept both p- and dev-p- prefixes
	return strings.HasPrefix(projectID, "p-") || strings.HasPrefix(projectID, "dev-p-")
}

// GetDefaultRegion returns the default region for deployments and storage operations
func (c *Config) GetDefaultRegion() string {
	if c.DefaultRegion != "" {
		return c.DefaultRegion
	}
	return "us-east-1" // Default fallback
}

// ensureConfigDir ensures the config directory exists
func ensureConfigDir() error {
	configPath := getConfigPath()
	configDir := filepath.Dir(configPath)
	return os.MkdirAll(configDir, 0755) //nolint:gosec // Config directory needs standard permissions
}

// getKeyPrefix returns the environment-specific key prefix
func getKeyPrefix(env Environment) string {
	if env == EnvProd {
		return ""
	}
	return string(env) + "-"
}

// GetEnvConfig returns the environment configuration
func (c *Config) GetEnvConfig() *EnvConfig {
	return c.envConfig
}

// GetLogLevel returns the configured log level as slog.Level
// Defaults to Info if not set or invalid
func (c *Config) GetLogLevel() slog.Level {
	if c.LogLevel == "" {
		return slog.LevelInfo
	}

	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
