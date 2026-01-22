package projectconfig

import (
	"fmt"
	"os"

	"github.com/spf13/viper"
)

// Default values matching Python implementation
var (
	DefaultPythonVersion             = "3.11"
	DefaultDockerBaseImageURL        = "debian:bookworm-slim"
	DefaultInclude                   = []string{"./*", "main.py", "cerebrium.toml"}
	DefaultExclude                   = []string{".*"}
	DefaultDisableAuth               = true
	DefaultEntrypoint                = []string{"uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"}
	DefaultPort                      = 8000
	DefaultHealthcheckEndpoint       = ""
	DefaultReadycheckEndpoint        = ""
	DefaultProvider                  = "aws"
	DefaultEvaluationIntervalSeconds = 30
	DefaultLoadBalancingAlgorithm    = "round-robin"
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
	config.DeprecationWarnings = []string{}

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

	// Parse new runtime sections (cortex, python, docker)
	if err := parseRuntimeSections(v, &config); err != nil {
		return nil, err
	}

	// Check for deprecated deployment fields and add warnings
	checkDeprecatedDeploymentFields(v, &config)

	// Apply defaults for missing fields
	applyDefaults(&config)

	return &config, nil
}

// parseRuntimeSections parses all runtime sections from the config
func parseRuntimeSections(v *viper.Viper, config *ProjectConfig) error {
	// Parse cortex runtime section
	if v.IsSet("cerebrium.runtime.cortex") {
		var cortexRuntime CortexRuntimeConfig
		if err := v.UnmarshalKey("cerebrium.runtime.cortex", &cortexRuntime); err != nil {
			return fmt.Errorf("failed to parse cortex runtime config: %w", err)
		}
		config.CortexRuntime = &cortexRuntime
	}

	// Parse python runtime section
	if v.IsSet("cerebrium.runtime.python") {
		var pythonRuntime PythonRuntimeConfig
		if err := v.UnmarshalKey("cerebrium.runtime.python", &pythonRuntime); err != nil {
			return fmt.Errorf("failed to parse python runtime config: %w", err)
		}
		config.PythonRuntime = &pythonRuntime
	}

	// Parse docker runtime section
	if v.IsSet("cerebrium.runtime.docker") {
		var dockerRuntime DockerRuntimeConfig
		if err := v.UnmarshalKey("cerebrium.runtime.docker", &dockerRuntime); err != nil {
			return fmt.Errorf("failed to parse docker runtime config: %w", err)
		}
		config.DockerRuntime = &dockerRuntime
	}

	// Parse deprecated custom runtime section
	if v.IsSet("cerebrium.runtime.custom") {
		var customRuntime CustomRuntimeConfig
		if err := v.UnmarshalKey("cerebrium.runtime.custom", &customRuntime); err != nil {
			return fmt.Errorf("failed to parse custom runtime config: %w", err)
		}
		config.CustomRuntime = &customRuntime

		// Add deprecation warning and migrate to appropriate runtime type
		migrateCustomRuntime(config)
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

			// Check for model_name (e.g., "arcana", "mist")
			if v.IsSet(key + ".model_name") {
				modelName := v.GetString(key + ".model_name")
				partnerConfig.ModelName = &modelName
			}

			// Check for language (e.g., "en", "es")
			if v.IsSet(key + ".language") {
				language := v.GetString(key + ".language")
				partnerConfig.Language = &language
			}

			config.PartnerService = partnerConfig
			break // Only one partner service at a time
		}
	}

	return nil
}

// migrateCustomRuntime migrates deprecated [cerebrium.runtime.custom] to the appropriate new runtime type
func migrateCustomRuntime(config *ProjectConfig) {
	if config.CustomRuntime == nil {
		return
	}

	// Determine if this is a docker or python runtime based on dockerfile_path
	if config.CustomRuntime.DockerfilePath != "" {
		// This is a docker runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			"[cerebrium.runtime.custom] is deprecated. Please migrate to [cerebrium.runtime.docker]:\n"+
				"  [cerebrium.runtime.docker]\n"+
				"  dockerfile_path = \""+config.CustomRuntime.DockerfilePath+"\"")
	} else {
		// This is a python runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			"[cerebrium.runtime.custom] is deprecated. Please migrate to [cerebrium.runtime.python]:\n"+
				"  [cerebrium.runtime.python]\n"+
				"  entrypoint = [\"uvicorn\", \"app.main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]")
	}
}

// checkDeprecatedDeploymentFields checks for deprecated fields in the deployment section
func checkDeprecatedDeploymentFields(v *viper.Viper, config *ProjectConfig) {
	deprecatedFields := []struct {
		key     string
		name    string
		newLoc  string
	}{
		{"cerebrium.deployment.python_version", "python_version", "[cerebrium.runtime.cortex] or [cerebrium.runtime.python]"},
		{"cerebrium.deployment.docker_base_image_url", "docker_base_image_url", "[cerebrium.runtime.cortex] or [cerebrium.runtime.python]"},
		{"cerebrium.deployment.shell_commands", "shell_commands", "[cerebrium.runtime.cortex] or [cerebrium.runtime.python]"},
		{"cerebrium.deployment.pre_build_commands", "pre_build_commands", "[cerebrium.runtime.cortex] or [cerebrium.runtime.python]"},
		{"cerebrium.deployment.use_uv", "use_uv", "[cerebrium.runtime.cortex] or [cerebrium.runtime.python]"},
	}

	for _, field := range deprecatedFields {
		if v.IsSet(field.key) {
			config.DeprecationWarnings = append(config.DeprecationWarnings,
				fmt.Sprintf("[cerebrium.deployment].%s is deprecated. Please move to %s", field.name, field.newLoc))
		}
	}
}

// applyDefaults sets default values for fields that weren't specified in the config
func applyDefaults(config *ProjectConfig) {
	// Apply deployment defaults (app-level only)
	if len(config.Deployment.Include) == 0 {
		config.Deployment.Include = DefaultInclude
	}
	if len(config.Deployment.Exclude) == 0 {
		config.Deployment.Exclude = DefaultExclude
	}
	if config.Deployment.DisableAuth == nil {
		disableAuth := DefaultDisableAuth
		config.Deployment.DisableAuth = &disableAuth
	}

	// Apply hardware defaults
	if config.Hardware.Provider == nil {
		config.Hardware.Provider = &DefaultProvider
	}

	// Apply scaling defaults
	if config.Scaling.EvaluationIntervalSeconds == nil {
		config.Scaling.EvaluationIntervalSeconds = &DefaultEvaluationIntervalSeconds
	}
	if config.Scaling.LoadBalancingAlgorithm == nil {
		config.Scaling.LoadBalancingAlgorithm = &DefaultLoadBalancingAlgorithm
	}

	// Apply cortex runtime defaults
	if config.CortexRuntime != nil {
		if config.CortexRuntime.PythonVersion == "" {
			config.CortexRuntime.PythonVersion = DefaultPythonVersion
		}
		if config.CortexRuntime.DockerBaseImageURL == "" {
			config.CortexRuntime.DockerBaseImageURL = DefaultDockerBaseImageURL
		}
	}

	// Apply python runtime defaults
	if config.PythonRuntime != nil {
		if config.PythonRuntime.PythonVersion == "" {
			config.PythonRuntime.PythonVersion = DefaultPythonVersion
		}
		if config.PythonRuntime.DockerBaseImageURL == "" {
			config.PythonRuntime.DockerBaseImageURL = DefaultDockerBaseImageURL
		}
		if len(config.PythonRuntime.Entrypoint) == 0 {
			config.PythonRuntime.Entrypoint = DefaultEntrypoint
		}
		if config.PythonRuntime.Port == 0 {
			config.PythonRuntime.Port = DefaultPort
		}
	}

	// Apply docker runtime defaults
	if config.DockerRuntime != nil {
		if config.DockerRuntime.Port == 0 {
			config.DockerRuntime.Port = DefaultPort
		}
	}

	// Apply custom runtime defaults (deprecated)
	if config.CustomRuntime != nil {
		if len(config.CustomRuntime.Entrypoint) == 0 {
			config.CustomRuntime.Entrypoint = DefaultEntrypoint
		}
		if config.CustomRuntime.Port == 0 {
			config.CustomRuntime.Port = DefaultPort
		}
	}
}
