package projectconfig

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Default values matching Python implementation
var (
	DefaultPythonVersion      = "3.11"
	DefaultDockerBaseImageURL = "debian:bookworm-slim"
	DefaultInclude            = []string{"./*", "main.py", "cerebrium.toml"}
	DefaultExclude            = []string{".*"}
	DefaultDisableAuth        = true
	DefaultEntrypoint         = []string{"uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"}
	DefaultPort               = 8000
	DefaultHealthcheckEndpoint = ""
	DefaultReadycheckEndpoint  = ""
)

// Load reads and parses the cerebrium.toml configuration file
func Load(configPath string) (*ProjectConfig, error) {
	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file not found: %s. Please run `cerebrium init` to create one", configPath)
	}

	// Create new viper instance for project config
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("toml")

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Check if 'cerebrium' key exists
	if !v.IsSet("cerebrium") {
		return nil, fmt.Errorf("'cerebrium' key not found in %s. Please ensure your config file is valid", configPath)
	}

	// Parse into config struct
	var config ProjectConfig

	// Parse deployment section (required)
	if err := v.UnmarshalKey("cerebrium.deployment", &config.Deployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment config: %w", err)
	}

	// Parse hardware section
	if v.IsSet("cerebrium.hardware") {
		if err := v.UnmarshalKey("cerebrium.hardware", &config.Hardware); err != nil {
			return nil, fmt.Errorf("failed to parse hardware config: %w", err)
		}
	}

	// Parse scaling section
	if v.IsSet("cerebrium.scaling") {
		if err := v.UnmarshalKey("cerebrium.scaling", &config.Scaling); err != nil {
			return nil, fmt.Errorf("failed to parse scaling config: %w", err)
		}
	}

	// Parse dependencies section
	if v.IsSet("cerebrium.dependencies") {
		if err := v.UnmarshalKey("cerebrium.dependencies", &config.Dependencies); err != nil {
			return nil, fmt.Errorf("failed to parse dependencies config: %w", err)
		}
	}

	// Parse custom runtime section
	if v.IsSet("cerebrium.runtime.custom") {
		var customRuntime CustomRuntimeConfig
		if err := v.UnmarshalKey("cerebrium.runtime.custom", &customRuntime); err != nil {
			return nil, fmt.Errorf("failed to parse custom runtime config: %w", err)
		}
		config.CustomRuntime = &customRuntime
	}

	// Parse partner service sections (deepgram, rime, etc.)
	partnerNames := []string{"deepgram", "rime"}
	for _, partner := range partnerNames {
		key := fmt.Sprintf("cerebrium.runtime.%s", partner)
		if v.IsSet(key) {
			partnerConfig := &PartnerServiceConfig{Name: partner}

			// Check if it's a map with port
			if v.IsSet(key + ".port") {
				port := v.GetInt(key + ".port")
				partnerConfig.Port = &port
			}

			config.PartnerService = partnerConfig
			break // Only one partner service at a time
		}
	}

	// Apply defaults for missing fields
	applyDefaults(&config)

	return &config, nil
}

// applyDefaults sets default values for fields that weren't specified in the config
func applyDefaults(config *ProjectConfig) {
	// Apply deployment defaults
	if config.Deployment.PythonVersion == "" {
		config.Deployment.PythonVersion = DefaultPythonVersion
	}
	if config.Deployment.DockerBaseImageURL == "" {
		config.Deployment.DockerBaseImageURL = DefaultDockerBaseImageURL
	}
	if len(config.Deployment.Include) == 0 {
		config.Deployment.Include = DefaultInclude
	}
	if len(config.Deployment.Exclude) == 0 {
		config.Deployment.Exclude = DefaultExclude
	}

	// Apply custom runtime defaults
	if config.CustomRuntime != nil {
		if len(config.CustomRuntime.Entrypoint) == 0 {
			config.CustomRuntime.Entrypoint = DefaultEntrypoint
		}
		if config.CustomRuntime.Port == 0 {
			config.CustomRuntime.Port = DefaultPort
		}
		if config.CustomRuntime.HealthcheckEndpoint == "" {
			config.CustomRuntime.HealthcheckEndpoint = DefaultHealthcheckEndpoint
		}
		if config.CustomRuntime.ReadycheckEndpoint == "" {
			config.CustomRuntime.ReadycheckEndpoint = DefaultReadycheckEndpoint
		}
	}

	// Note: DisableAuth defaults to false (Go zero value), matching Python's True is intentional difference
	// The Python default of True is for backwards compatibility
}
