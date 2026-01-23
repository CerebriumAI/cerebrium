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

	// Check for deprecated top-level dependencies and add warnings
	checkDeprecatedDependencies(v, &config)

	// Apply defaults for missing fields
	applyDefaults(&config)

	return &config, nil
}

// parseRuntimeSections parses the runtime section from the config
// The CLI accepts any [cerebrium.runtime.<name>] section and passes it to the backend
func parseRuntimeSections(v *viper.Viper, config *ProjectConfig) error {
	// Get all keys under cerebrium.runtime
	runtimeSection := v.GetStringMap("cerebrium.runtime")
	if len(runtimeSection) == 0 {
		return nil
	}

	// Find the runtime type(s) - there should be exactly one
	var runtimeTypes []string
	for key := range runtimeSection {
		runtimeTypes = append(runtimeTypes, key)
	}

	if len(runtimeTypes) > 1 {
		return fmt.Errorf("only one runtime type can be specified, found: %v", runtimeTypes)
	}

	if len(runtimeTypes) == 1 {
		runtimeType := runtimeTypes[0]
		key := fmt.Sprintf("cerebrium.runtime.%s", runtimeType)

		// Parse the runtime params as a generic map
		params := v.GetStringMap(key)

		// Convert the params to map[string]any for proper type handling
		runtimeParams := make(map[string]any)
		for k := range params {
			// Re-fetch with proper type preservation
			paramKey := fmt.Sprintf("%s.%s", key, k)
			runtimeParams[k] = v.Get(paramKey)
		}

		config.Runtime = &RuntimeConfig{
			Type:   runtimeType,
			Params: runtimeParams,
		}

		// Add deprecation warning for [cerebrium.runtime.custom]
		if runtimeType == "custom" {
			addCustomRuntimeDeprecationWarning(config)
		}
	}

	return nil
}

// addCustomRuntimeDeprecationWarning adds a deprecation warning for [cerebrium.runtime.custom]
func addCustomRuntimeDeprecationWarning(config *ProjectConfig) {
	if config.Runtime == nil || config.Runtime.Type != "custom" {
		return
	}

	// Determine if this is a docker or python runtime based on dockerfile_path
	if config.Runtime.GetDockerfilePath() != "" {
		// This is a docker runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			"[cerebrium.runtime.custom] is deprecated. Please migrate to [cerebrium.runtime.docker]:\n"+
				"  [cerebrium.runtime.docker]\n"+
				"  dockerfile_path = \""+config.Runtime.GetDockerfilePath()+"\"")
	} else {
		// This is a python runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			"[cerebrium.runtime.custom] is deprecated. Please migrate to [cerebrium.runtime.python]:\n"+
				"  [cerebrium.runtime.python]\n"+
				"  entrypoint = [\"uvicorn\", \"app.main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]")
	}
}

// checkDeprecatedDependencies checks for deprecated top-level dependencies section
func checkDeprecatedDependencies(_ *viper.Viper, config *ProjectConfig) {
	if !config.HasTopLevelDependencies() {
		return
	}

	// Determine the target runtime section for the deprecation message
	runtimeType := config.GetRuntimeType()
	if runtimeType == "docker" {
		// Docker runtime doesn't use dependencies - don't suggest moving them
		return
	}

	config.DeprecationWarnings = append(config.DeprecationWarnings,
		fmt.Sprintf("[cerebrium.dependencies] is deprecated. Please move to [cerebrium.runtime.%s.dependencies.*]:\n"+
			"  [cerebrium.runtime.%s.dependencies.pip]\n"+
			"  torch = \"2.0.0\"\n"+
			"  \n"+
			"  [cerebrium.runtime.%s.dependencies.apt]\n"+
			"  ffmpeg = \"\"",
			runtimeType, runtimeType, runtimeType))
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

	// Runtime defaults are not applied here - the backend handles validation
	// and defaults for runtime-specific parameters. This allows new runtimes
	// to be added without modifying the CLI.
}
