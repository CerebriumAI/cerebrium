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

	// Determine which format is being used
	// Legacy format: [cerebrium.deployment], [cerebrium.hardware], etc.
	// New format: [deployment], [hardware], etc.
	useLegacyFormat := v.IsSet("cerebrium")
	useNewFormat := v.IsSet("deployment") || v.IsSet("hardware") || v.IsSet("scaling") || v.IsSet("dependencies") || v.IsSet("runtime")

	if useLegacyFormat && useNewFormat {
		return nil, fmt.Errorf("cannot mix [cerebrium.*] format with new format. Please use one or the other")
	}

	if !useLegacyFormat && !useNewFormat {
		return nil, fmt.Errorf("no valid configuration found in %s. Expected [deployment] section", configPath)
	}

	// Parse into config struct
	var config ProjectConfig
	config.DeprecationWarnings = []string{}

	// Determine the key prefix based on format
	prefix := ""
	if useLegacyFormat {
		prefix = "cerebrium."
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			"[cerebrium.*] prefix is deprecated. Please remove the 'cerebrium.' prefix from all sections:\n"+
				"  [cerebrium.deployment] → [deployment]\n"+
				"  [cerebrium.hardware] → [hardware]\n"+
				"  [cerebrium.scaling] → [scaling]\n"+
				"  [cerebrium.runtime.cortex] → [runtime.cortex]")
	}

	// Parse deployment section (required)
	deploymentKey := prefix + "deployment"
	if !v.IsSet(deploymentKey) {
		return nil, fmt.Errorf("[deployment] section not found in %s", configPath)
	}
	if err := v.UnmarshalKey(deploymentKey, &config.Deployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment config: %w", err)
	}

	// Parse hardware section
	hardwareKey := prefix + "hardware"
	if v.IsSet(hardwareKey) {
		if err := v.UnmarshalKey(hardwareKey, &config.Hardware); err != nil {
			return nil, fmt.Errorf("failed to parse hardware config: %w", err)
		}
	}

	// Parse scaling section
	scalingKey := prefix + "scaling"
	if v.IsSet(scalingKey) {
		if err := v.UnmarshalKey(scalingKey, &config.Scaling); err != nil {
			return nil, fmt.Errorf("failed to parse scaling config: %w", err)
		}
	}

	// Parse dependencies section
	dependenciesKey := prefix + "dependencies"
	if v.IsSet(dependenciesKey) {
		if err := v.UnmarshalKey(dependenciesKey, &config.Dependencies); err != nil {
			return nil, fmt.Errorf("failed to parse dependencies config: %w", err)
		}
	}

	// Parse runtime sections (cortex, python, docker, etc.)
	if err := parseRuntimeSections(v, &config, prefix); err != nil {
		return nil, err
	}

	// Check for deprecated deployment fields and add warnings
	checkDeprecatedDeploymentFields(v, &config, prefix)

	// Check for deprecated top-level dependencies and add warnings
	checkDeprecatedDependencies(&config, prefix)

	// Apply defaults for missing fields
	applyDefaults(&config)

	return &config, nil
}

// parseRuntimeSections parses the runtime section from the config
// The CLI accepts any [runtime.<name>] section and passes it to the backend
func parseRuntimeSections(v *viper.Viper, config *ProjectConfig, prefix string) error {
	// Get all keys under runtime (with appropriate prefix)
	runtimeKey := prefix + "runtime"
	runtimeSection := v.GetStringMap(runtimeKey)
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
		key := fmt.Sprintf("%sruntime.%s", prefix, runtimeType)

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

		// Add deprecation warning for [runtime.custom]
		if runtimeType == "custom" {
			addCustomRuntimeDeprecationWarning(config, prefix)
		}
	}

	return nil
}

// addCustomRuntimeDeprecationWarning adds a deprecation warning for [runtime.custom]
func addCustomRuntimeDeprecationWarning(config *ProjectConfig, prefix string) {
	if config.Runtime == nil || config.Runtime.Type != "custom" {
		return
	}

	// Determine if this is a docker or python runtime based on dockerfile_path
	if config.Runtime.GetDockerfilePath() != "" {
		// This is a docker runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			fmt.Sprintf("[%sruntime.custom] is deprecated. Please migrate to [%sruntime.docker]:\n"+
				"  [%sruntime.docker]\n"+
				"  dockerfile_path = \"%s\"", prefix, prefix, prefix, config.Runtime.GetDockerfilePath()))
	} else {
		// This is a python runtime
		config.DeprecationWarnings = append(config.DeprecationWarnings,
			fmt.Sprintf("[%sruntime.custom] is deprecated. Please migrate to [%sruntime.python]:\n"+
				"  [%sruntime.python]\n"+
				"  entrypoint = [\"uvicorn\", \"app.main:app\", \"--host\", \"0.0.0.0\", \"--port\", \"8000\"]", prefix, prefix, prefix))
	}
}

// checkDeprecatedDependencies checks for deprecated top-level dependencies section
func checkDeprecatedDependencies(config *ProjectConfig, prefix string) {
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
		fmt.Sprintf("[%sdependencies] is deprecated. Please move to [%sruntime.%s.deps.*]:\n"+
			"  [%sruntime.%s.deps.pip]\n"+
			"  torch = \"2.0.0\"\n"+
			"  \n"+
			"  [%sruntime.%s.deps.apt]\n"+
			"  ffmpeg = \"\"",
			prefix, prefix, runtimeType, prefix, runtimeType, prefix, runtimeType))
}

// checkDeprecatedDeploymentFields checks for deprecated fields in the deployment section
func checkDeprecatedDeploymentFields(v *viper.Viper, config *ProjectConfig, prefix string) {
	deprecatedFields := []struct {
		name   string
		newLoc string
	}{
		{"python_version", "[runtime.cortex] or [runtime.python]"},
		{"docker_base_image_url", "[runtime.cortex] or [runtime.python]"},
		{"shell_commands", "[runtime.cortex] or [runtime.python]"},
		{"pre_build_commands", "[runtime.cortex] or [runtime.python]"},
		{"use_uv", "[runtime.cortex] or [runtime.python]"},
	}

	for _, field := range deprecatedFields {
		key := prefix + "deployment." + field.name
		if v.IsSet(key) {
			config.DeprecationWarnings = append(config.DeprecationWarnings,
				fmt.Sprintf("[%sdeployment].%s is deprecated. Please move to %s", prefix, field.name, field.newLoc))
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
