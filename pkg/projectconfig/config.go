package projectconfig

import "maps"

// ProjectConfig represents the complete cerebrium.toml configuration
type ProjectConfig struct {
	Deployment   DeploymentConfig   `mapstructure:"deployment" toml:"deployment"`
	Hardware     HardwareConfig     `mapstructure:"hardware" toml:"hardware"`
	Scaling      ScalingConfig      `mapstructure:"scaling" toml:"scaling"`
	Dependencies DependenciesConfig `mapstructure:"dependencies" toml:"dependencies"`

	// Runtime holds the generic runtime configuration from [cerebrium.runtime.<name>]
	// The CLI accepts any runtime type and passes parameters to the backend for validation
	Runtime *RuntimeConfig `mapstructure:"-" toml:"-"`

	// DeprecationWarnings contains warnings about deprecated config fields
	DeprecationWarnings []string `mapstructure:"-" toml:"-"`
}

// RuntimeConfig represents a generic runtime configuration
// The CLI accepts any [cerebrium.runtime.<name>] section and passes it to the backend
type RuntimeConfig struct {
	// Type is the runtime name (cortex, python, docker, rime, deepgram, etc.)
	Type string `mapstructure:"-" toml:"-"`

	// Params contains all parameters from the runtime section
	// The backend validates these parameters based on the runtime type
	Params map[string]any `mapstructure:",remain" toml:",inline"`
}

// GetRuntimeType returns the active runtime type name for this configuration
// Returns "cortex" as default if no runtime is specified
func (pc *ProjectConfig) GetRuntimeType() string {
	if pc.Runtime != nil && pc.Runtime.Type != "" {
		return pc.Runtime.Type
	}
	// Default to cortex
	return "cortex"
}

// GetEffectiveDependencies merges top-level and runtime-specific dependencies
// Runtime dependencies take precedence over top-level dependencies (per-package)
func (pc *ProjectConfig) GetEffectiveDependencies() DependenciesConfig {
	// Start with top-level dependencies as base
	result := DependenciesConfig{
		Pip:   make(map[string]string),
		Conda: make(map[string]string),
		Apt:   make(map[string]string),
		Paths: pc.Dependencies.Paths,
	}

	// Copy top-level dependencies
	maps.Copy(result.Pip, pc.Dependencies.Pip)
	maps.Copy(result.Conda, pc.Dependencies.Conda)
	maps.Copy(result.Apt, pc.Dependencies.Apt)

	// Merge runtime dependencies on top (runtime wins per-package)
	if pc.Runtime != nil {
		runtimeDeps := pc.Runtime.GetDependencies()
		if runtimeDeps != nil {
			// Merge pip, conda, apt (runtime wins per-package)
			maps.Copy(result.Pip, runtimeDeps.Pip)
			maps.Copy(result.Conda, runtimeDeps.Conda)
			maps.Copy(result.Apt, runtimeDeps.Apt)

			// Runtime paths override top-level paths
			if runtimeDeps.Paths.Pip != "" {
				result.Paths.Pip = runtimeDeps.Paths.Pip
			}
			if runtimeDeps.Paths.Conda != "" {
				result.Paths.Conda = runtimeDeps.Paths.Conda
			}
			if runtimeDeps.Paths.Apt != "" {
				result.Paths.Apt = runtimeDeps.Paths.Apt
			}
		}
	}

	return result
}

// HasTopLevelDependencies returns true if any top-level dependencies are specified
func (pc *ProjectConfig) HasTopLevelDependencies() bool {
	return len(pc.Dependencies.Pip) > 0 ||
		len(pc.Dependencies.Conda) > 0 ||
		len(pc.Dependencies.Apt) > 0 ||
		pc.Dependencies.Paths.Pip != "" ||
		pc.Dependencies.Paths.Conda != "" ||
		pc.Dependencies.Paths.Apt != ""
}

// GetEffectiveShellCommands returns shell commands from runtime section if present,
// otherwise falls back to deprecated deployment section
func (pc *ProjectConfig) GetEffectiveShellCommands() []string {
	if pc.Runtime != nil {
		if cmds := pc.Runtime.GetShellCommands(); len(cmds) > 0 {
			return cmds
		}
	}
	return pc.Deployment.ShellCommands
}

// GetEffectivePreBuildCommands returns pre-build commands from runtime section if present,
// otherwise falls back to deprecated deployment section
func (pc *ProjectConfig) GetEffectivePreBuildCommands() []string {
	if pc.Runtime != nil {
		if cmds := pc.Runtime.GetPreBuildCommands(); len(cmds) > 0 {
			return cmds
		}
	}
	return pc.Deployment.PreBuildCommands
}

// Helper methods to access common runtime parameters that the CLI needs locally

// GetDockerfilePath returns the dockerfile_path from runtime params, if present
func (rc *RuntimeConfig) GetDockerfilePath() string {
	if rc == nil || rc.Params == nil {
		return ""
	}
	if path, ok := rc.Params["dockerfile_path"].(string); ok {
		return path
	}
	return ""
}

// GetEntrypoint returns the entrypoint from runtime params, if present
func (rc *RuntimeConfig) GetEntrypoint() []string {
	if rc == nil || rc.Params == nil {
		return nil
	}
	if entrypoint, ok := rc.Params["entrypoint"].([]any); ok {
		result := make([]string, len(entrypoint))
		for i, v := range entrypoint {
			if s, ok := v.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	// Handle case where it's already []string (from test setup)
	if entrypoint, ok := rc.Params["entrypoint"].([]string); ok {
		return entrypoint
	}
	return nil
}

// GetPort returns the port from runtime params, if present (defaults to 8000)
func (rc *RuntimeConfig) GetPort() int {
	if rc == nil || rc.Params == nil {
		return DefaultPort
	}
	if port, ok := rc.Params["port"].(int); ok {
		return port
	}
	if port, ok := rc.Params["port"].(int64); ok {
		return int(port)
	}
	if port, ok := rc.Params["port"].(float64); ok {
		return int(port)
	}
	return DefaultPort
}

// GetShellCommands returns shell_commands from runtime params, if present
func (rc *RuntimeConfig) GetShellCommands() []string {
	return rc.getStringSlice("shell_commands")
}

// GetPreBuildCommands returns pre_build_commands from runtime params, if present
func (rc *RuntimeConfig) GetPreBuildCommands() []string {
	return rc.getStringSlice("pre_build_commands")
}

// getStringSlice is a helper to extract []string from runtime params
func (rc *RuntimeConfig) getStringSlice(key string) []string {
	if rc == nil || rc.Params == nil {
		return nil
	}
	if arr, ok := rc.Params[key].([]any); ok {
		result := make([]string, len(arr))
		for i, v := range arr {
			if s, ok := v.(string); ok {
				result[i] = s
			}
		}
		return result
	}
	// Handle case where it's already []string (from test setup)
	if arr, ok := rc.Params[key].([]string); ok {
		return arr
	}
	return nil
}

// GetDependencies extracts dependencies from runtime params, if present
// This parses the [cerebrium.runtime.<type>.dependencies.*] sections
func (rc *RuntimeConfig) GetDependencies() *DependenciesConfig {
	if rc == nil || rc.Params == nil {
		return nil
	}

	depsRaw, ok := rc.Params["deps"]
	if !ok {
		return nil
	}

	depsMap, ok := depsRaw.(map[string]any)
	if !ok {
		return nil
	}

	deps := &DependenciesConfig{
		Pip:   make(map[string]string),
		Conda: make(map[string]string),
		Apt:   make(map[string]string),
	}

	// Parse pip dependencies
	if pipRaw, ok := depsMap["pip"].(map[string]any); ok {
		for pkg, ver := range pipRaw {
			deps.Pip[pkg] = anyToString(ver)
		}
	}

	// Parse conda dependencies
	if condaRaw, ok := depsMap["conda"].(map[string]any); ok {
		for pkg, ver := range condaRaw {
			deps.Conda[pkg] = anyToString(ver)
		}
	}

	// Parse apt dependencies
	if aptRaw, ok := depsMap["apt"].(map[string]any); ok {
		for pkg, ver := range aptRaw {
			deps.Apt[pkg] = anyToString(ver)
		}
	}

	// Parse paths section
	if pathsRaw, ok := depsMap["paths"].(map[string]any); ok {
		if pipPath, ok := pathsRaw["pip"].(string); ok {
			deps.Paths.Pip = pipPath
		}
		if condaPath, ok := pathsRaw["conda"].(string); ok {
			deps.Paths.Conda = condaPath
		}
		if aptPath, ok := pathsRaw["apt"].(string); ok {
			deps.Paths.Apt = aptPath
		}
	}

	return deps
}

// anyToString converts an any value to string for dependency versions
func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		return ""
	}
}

// DeploymentConfig represents the [cerebrium.deployment] section
// Note: python_version, docker_base_image_url, shell_commands, pre_build_commands, use_uv
// are deprecated here and should be moved to the appropriate runtime section
type DeploymentConfig struct {
	Name                            string   `mapstructure:"name" toml:"name,omitempty"`
	Include                         []string `mapstructure:"include" toml:"include,omitempty"`
	Exclude                         []string `mapstructure:"exclude" toml:"exclude,omitempty"`
	DisableAuth                     *bool    `mapstructure:"disable_auth" toml:"disable_auth,omitempty"`
	DeploymentInitializationTimeout *int     `mapstructure:"deployment_initialization_timeout" toml:"deployment_initialization_timeout,omitempty"`

	// Deprecated fields - these should be in runtime sections
	PythonVersion      string   `mapstructure:"python_version" toml:"python_version,omitempty"`
	DockerBaseImageURL string   `mapstructure:"docker_base_image_url" toml:"docker_base_image_url,omitempty"`
	ShellCommands      []string `mapstructure:"shell_commands" toml:"shell_commands,omitempty"`
	PreBuildCommands   []string `mapstructure:"pre_build_commands" toml:"pre_build_commands,omitempty"`
	UseUv              *bool    `mapstructure:"use_uv" toml:"use_uv,omitempty"`
}


// HardwareConfig represents the [cerebrium.hardware] section
type HardwareConfig struct {
	CPU      *float64 `mapstructure:"cpu" toml:"cpu,omitempty"`
	Memory   *float64 `mapstructure:"memory" toml:"memory,omitempty"`
	Compute  *string  `mapstructure:"compute" toml:"compute,omitempty"`
	GPUCount *int     `mapstructure:"gpu_count" toml:"gpu_count,omitempty"`
	Provider *string  `mapstructure:"provider" toml:"provider,omitempty"`
	Region   *string  `mapstructure:"region" toml:"region,omitempty"`
}

// ScalingConfig represents the [cerebrium.scaling] section
type ScalingConfig struct {
	MinReplicas               *int    `mapstructure:"min_replicas" toml:"min_replicas,omitempty"`
	MaxReplicas               *int    `mapstructure:"max_replicas" toml:"max_replicas,omitempty"`
	Cooldown                  *int    `mapstructure:"cooldown" toml:"cooldown,omitempty"`
	ReplicaConcurrency        *int    `mapstructure:"replica_concurrency" toml:"replica_concurrency,omitempty"`
	ResponseGracePeriod       *int    `mapstructure:"response_grace_period" toml:"response_grace_period,omitempty"`
	ScalingMetric             *string `mapstructure:"scaling_metric" toml:"scaling_metric,omitempty"`
	ScalingTarget             *int    `mapstructure:"scaling_target" toml:"scaling_target,omitempty"`
	ScalingBuffer             *int    `mapstructure:"scaling_buffer" toml:"scaling_buffer,omitempty"`
	RollOutDurationSeconds    *int    `mapstructure:"roll_out_duration_seconds" toml:"roll_out_duration_seconds,omitempty"`
	EvaluationIntervalSeconds *int    `mapstructure:"evaluation_interval_seconds" toml:"evaluation_interval_seconds,omitempty"`
	LoadBalancingAlgorithm    *string `mapstructure:"load_balancing_algorithm" toml:"load_balancing_algorithm,omitempty"`
}

// DependenciesConfig represents the [cerebrium.dependencies.*] sections
type DependenciesConfig struct {
	Pip   map[string]string     `mapstructure:"pip" toml:"pip,omitempty"`
	Conda map[string]string     `mapstructure:"conda" toml:"conda,omitempty"`
	Apt   map[string]string     `mapstructure:"apt" toml:"apt,omitempty"`
	Paths DependencyPathsConfig `mapstructure:"paths" toml:"paths,omitempty"`
}

// DependencyPathsConfig represents file paths for dependency files
type DependencyPathsConfig struct {
	Pip   string `mapstructure:"pip" toml:"pip,omitempty"`
	Conda string `mapstructure:"conda" toml:"conda,omitempty"`
	Apt   string `mapstructure:"apt" toml:"apt,omitempty"`
}


// ToPayload converts the project config to an API payload
func (pc *ProjectConfig) ToPayload() map[string]any {
	payload := make(map[string]any)

	// Deployment fields (app-level)
	payload["name"] = pc.Deployment.Name
	payload["include"] = pc.Deployment.Include
	payload["exclude"] = pc.Deployment.Exclude

	if pc.Deployment.DisableAuth != nil {
		payload["disableAuth"] = *pc.Deployment.DisableAuth
	}
	if pc.Deployment.DeploymentInitializationTimeout != nil {
		payload["deploymentInitializationTimeout"] = *pc.Deployment.DeploymentInitializationTimeout
	}

	// Hardware fields
	if pc.Hardware.CPU != nil {
		payload["cpu"] = *pc.Hardware.CPU
	}
	if pc.Hardware.Memory != nil {
		payload["memory"] = *pc.Hardware.Memory
	}
	if pc.Hardware.Compute != nil {
		payload["compute"] = *pc.Hardware.Compute
	}
	if pc.Hardware.GPUCount != nil && pc.Hardware.Compute != nil && *pc.Hardware.Compute != "CPU" {
		payload["gpuCount"] = *pc.Hardware.GPUCount
	}
	if pc.Hardware.Provider != nil {
		payload["provider"] = *pc.Hardware.Provider
	}
	if pc.Hardware.Region != nil {
		payload["region"] = *pc.Hardware.Region
	}

	// Scaling fields
	if pc.Scaling.MinReplicas != nil {
		payload["minReplicaCount"] = *pc.Scaling.MinReplicas
	}
	if pc.Scaling.MaxReplicas != nil {
		payload["maxReplicaCount"] = *pc.Scaling.MaxReplicas
	}
	if pc.Scaling.Cooldown != nil {
		payload["cooldownPeriodSeconds"] = *pc.Scaling.Cooldown
	}
	if pc.Scaling.ReplicaConcurrency != nil {
		payload["replicaConcurrency"] = *pc.Scaling.ReplicaConcurrency
	}
	if pc.Scaling.ResponseGracePeriod != nil {
		payload["responseGracePeriodSeconds"] = *pc.Scaling.ResponseGracePeriod
	}
	if pc.Scaling.ScalingMetric != nil {
		payload["scalingMetric"] = *pc.Scaling.ScalingMetric
	}
	if pc.Scaling.ScalingTarget != nil {
		payload["scalingTarget"] = *pc.Scaling.ScalingTarget
	}
	if pc.Scaling.ScalingBuffer != nil {
		payload["scalingBuffer"] = *pc.Scaling.ScalingBuffer
	}
	if pc.Scaling.RollOutDurationSeconds != nil {
		payload["rollOutDurationSeconds"] = *pc.Scaling.RollOutDurationSeconds
	}
	if pc.Scaling.EvaluationIntervalSeconds != nil {
		payload["evaluationIntervalSeconds"] = *pc.Scaling.EvaluationIntervalSeconds
	}
	if pc.Scaling.LoadBalancingAlgorithm != nil {
		payload["loadBalancingAlgorithm"] = *pc.Scaling.LoadBalancingAlgorithm
	}

	// Runtime configuration - pass through to backend
	pc.addRuntimePayload(payload)

	return payload
}

// addRuntimePayload adds runtime configuration to the payload
// The CLI passes runtime parameters to the backend for validation
func (pc *ProjectConfig) addRuntimePayload(payload map[string]any) {
	runtimeType := pc.GetRuntimeType()
	payload["runtime"] = runtimeType

	// If we have runtime params, pass them through to the backend
	if pc.Runtime != nil && pc.Runtime.Params != nil {
		for key, value := range pc.Runtime.Params {
			// Convert snake_case keys to API payload keys
			apiKey := toAPIKey(key)
			payload[apiKey] = value
		}
	}

	// Apply deprecated deployment field fallbacks for backwards compatibility
	// These are only applied if not already set by the runtime section
	pc.applyDeprecatedDeploymentFallbacks(payload)
}

// toAPIKey converts config keys to API payload keys
// Some keys have special mappings that differ from simple camelCase conversion
func toAPIKey(key string) string {
	// Special mappings for known fields
	specialMappings := map[string]string{
		"docker_base_image_url": "baseImage",
		"dockerfile_path":       "dockerfilePath",
	}

	if apiKey, ok := specialMappings[key]; ok {
		return apiKey
	}

	// Default to camelCase conversion
	return snakeToCamel(key)
}

// applyDeprecatedDeploymentFallbacks applies deprecated deployment fields if not set by runtime
func (pc *ProjectConfig) applyDeprecatedDeploymentFallbacks(payload map[string]any) {
	// Only apply fallbacks for non-docker runtimes (docker doesn't use these fields)
	if pc.Runtime != nil && pc.Runtime.GetDockerfilePath() != "" {
		return
	}

	// python_version fallback
	if _, ok := payload["pythonVersion"]; !ok {
		if pc.Deployment.PythonVersion != "" {
			payload["pythonVersion"] = pc.Deployment.PythonVersion
		} else {
			payload["pythonVersion"] = DefaultPythonVersion
		}
	}

	// docker_base_image_url fallback
	if _, ok := payload["baseImage"]; !ok {
		if pc.Deployment.DockerBaseImageURL != "" {
			payload["baseImage"] = pc.Deployment.DockerBaseImageURL
		} else {
			payload["baseImage"] = DefaultDockerBaseImageURL
		}
	}

	// shell_commands fallback
	if _, ok := payload["shellCommands"]; !ok {
		if len(pc.Deployment.ShellCommands) > 0 {
			payload["shellCommands"] = pc.Deployment.ShellCommands
		}
	}

	// pre_build_commands fallback
	if _, ok := payload["preBuildCommands"]; !ok {
		if len(pc.Deployment.PreBuildCommands) > 0 {
			payload["preBuildCommands"] = pc.Deployment.PreBuildCommands
		}
	}

	// use_uv fallback
	if _, ok := payload["useUv"]; !ok {
		if pc.Deployment.UseUv != nil {
			payload["useUv"] = *pc.Deployment.UseUv
		}
	}
}

// snakeToCamel converts snake_case to camelCase
func snakeToCamel(s string) string {
	result := make([]byte, 0, len(s))
	capitalizeNext := false

	for i := 0; i < len(s); i++ {
		if s[i] == '_' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext && s[i] >= 'a' && s[i] <= 'z' {
			result = append(result, s[i]-32) // Convert to uppercase
			capitalizeNext = false
		} else {
			result = append(result, s[i])
			capitalizeNext = false
		}
	}

	return string(result)
}
