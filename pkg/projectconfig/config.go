package projectconfig

// RuntimeType represents the type of runtime configuration
type RuntimeType string

const (
	RuntimeTypeCortex  RuntimeType = "cortex"
	RuntimeTypePython  RuntimeType = "python"
	RuntimeTypeDocker  RuntimeType = "docker"
	RuntimeTypeCustom  RuntimeType = "custom"  // deprecated alias
	RuntimeTypePartner RuntimeType = "partner" // for deepgram, rime, etc.
)

// ProjectConfig represents the complete cerebrium.toml configuration
type ProjectConfig struct {
	Deployment     DeploymentConfig      `mapstructure:"deployment" toml:"deployment"`
	Hardware       HardwareConfig        `mapstructure:"hardware" toml:"hardware"`
	Scaling        ScalingConfig         `mapstructure:"scaling" toml:"scaling"`
	Dependencies   DependenciesConfig    `mapstructure:"dependencies" toml:"dependencies"`
	CortexRuntime  *CortexRuntimeConfig  `mapstructure:"cortex" toml:"cortex,omitempty"`
	PythonRuntime  *PythonRuntimeConfig  `mapstructure:"python" toml:"python,omitempty"`
	DockerRuntime  *DockerRuntimeConfig  `mapstructure:"docker" toml:"docker,omitempty"`
	CustomRuntime  *CustomRuntimeConfig  `mapstructure:"custom" toml:"runtime,omitempty"` // deprecated
	PartnerService *PartnerServiceConfig `mapstructure:"partner" toml:"partner,omitempty"`

	// DeprecationWarnings contains warnings about deprecated config fields
	DeprecationWarnings []string `mapstructure:"-" toml:"-"`
}

// GetRuntimeType returns the active runtime type for this configuration
func (pc *ProjectConfig) GetRuntimeType() RuntimeType {
	if pc.DockerRuntime != nil {
		return RuntimeTypeDocker
	}
	if pc.PythonRuntime != nil {
		return RuntimeTypePython
	}
	if pc.CortexRuntime != nil {
		return RuntimeTypeCortex
	}
	if pc.CustomRuntime != nil {
		return RuntimeTypeCustom
	}
	if pc.PartnerService != nil {
		return RuntimeTypePartner
	}
	// Default to cortex
	return RuntimeTypeCortex
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

// CortexRuntimeConfig represents [cerebrium.runtime.cortex] (default Cerebrium managed Python)
type CortexRuntimeConfig struct {
	PythonVersion      string   `mapstructure:"python_version" toml:"python_version,omitempty"`
	DockerBaseImageURL string   `mapstructure:"docker_base_image_url" toml:"docker_base_image_url,omitempty"`
	ShellCommands      []string `mapstructure:"shell_commands" toml:"shell_commands,omitempty"`
	PreBuildCommands   []string `mapstructure:"pre_build_commands" toml:"pre_build_commands,omitempty"`
	UseUv              *bool    `mapstructure:"use_uv" toml:"use_uv,omitempty"`
}

// PythonRuntimeConfig represents [cerebrium.runtime.python] (custom Python ASGI app)
type PythonRuntimeConfig struct {
	PythonVersion       string   `mapstructure:"python_version" toml:"python_version,omitempty"`
	DockerBaseImageURL  string   `mapstructure:"docker_base_image_url" toml:"docker_base_image_url,omitempty"`
	ShellCommands       []string `mapstructure:"shell_commands" toml:"shell_commands,omitempty"`
	PreBuildCommands    []string `mapstructure:"pre_build_commands" toml:"pre_build_commands,omitempty"`
	UseUv               *bool    `mapstructure:"use_uv" toml:"use_uv,omitempty"`
	Entrypoint          []string `mapstructure:"entrypoint" toml:"entrypoint,omitempty"`
	Port                int      `mapstructure:"port" toml:"port,omitempty"`
	HealthcheckEndpoint string   `mapstructure:"healthcheck_endpoint" toml:"healthcheck_endpoint,omitempty"`
	ReadycheckEndpoint  string   `mapstructure:"readycheck_endpoint" toml:"readycheck_endpoint,omitempty"`
}

// DockerRuntimeConfig represents [cerebrium.runtime.docker] (custom Dockerfile)
type DockerRuntimeConfig struct {
	DockerfilePath      string   `mapstructure:"dockerfile_path" toml:"dockerfile_path,omitempty"`
	Entrypoint          []string `mapstructure:"entrypoint" toml:"entrypoint,omitempty"`
	Port                int      `mapstructure:"port" toml:"port,omitempty"`
	HealthcheckEndpoint string   `mapstructure:"healthcheck_endpoint" toml:"healthcheck_endpoint,omitempty"`
	ReadycheckEndpoint  string   `mapstructure:"readycheck_endpoint" toml:"readycheck_endpoint,omitempty"`
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

// CustomRuntimeConfig represents the [cerebrium.runtime.custom] section
type CustomRuntimeConfig struct {
	Entrypoint          []string `mapstructure:"entrypoint" toml:"entrypoint,omitempty"`
	Port                int      `mapstructure:"port" toml:"port,omitempty"`
	HealthcheckEndpoint string   `mapstructure:"healthcheck_endpoint" toml:"healthcheck_endpoint,omitempty"`
	ReadycheckEndpoint  string   `mapstructure:"readycheck_endpoint" toml:"readycheck_endpoint,omitempty"`
	DockerfilePath      string   `mapstructure:"dockerfile_path" toml:"dockerfile_path,omitempty"`
}

// PartnerServiceConfig represents partner service configurations
type PartnerServiceConfig struct {
	Name      string  `mapstructure:"name" toml:"name,omitempty"`
	Port      *int    `mapstructure:"port" toml:"port,omitempty"`
	ModelName *string `mapstructure:"model_name" toml:"model_name,omitempty"`
	Language  *string `mapstructure:"language" toml:"language,omitempty"`
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

	// Runtime configuration based on type
	pc.addRuntimePayload(payload)

	return payload
}

// addRuntimePayload adds runtime-specific fields to the payload
func (pc *ProjectConfig) addRuntimePayload(payload map[string]any) {
	runtimeType := pc.GetRuntimeType()

	switch runtimeType {
	case RuntimeTypeDocker:
		pc.addDockerRuntimePayload(payload)
	case RuntimeTypePython:
		pc.addPythonRuntimePayload(payload)
	case RuntimeTypeCortex:
		pc.addCortexRuntimePayload(payload)
	case RuntimeTypeCustom:
		pc.addCustomRuntimePayload(payload)
	case RuntimeTypePartner:
		pc.addPartnerRuntimePayload(payload)
	}
}

// addCortexRuntimePayload adds cortex runtime fields to the payload
func (pc *ProjectConfig) addCortexRuntimePayload(payload map[string]any) {
	payload["runtime"] = "cortex"

	// Get values from cortex runtime or fallback to deprecated deployment fields
	pythonVersion := DefaultPythonVersion
	baseImage := DefaultDockerBaseImageURL
	var shellCommands []string
	var preBuildCommands []string
	var useUv *bool

	if pc.CortexRuntime != nil {
		if pc.CortexRuntime.PythonVersion != "" {
			pythonVersion = pc.CortexRuntime.PythonVersion
		}
		if pc.CortexRuntime.DockerBaseImageURL != "" {
			baseImage = pc.CortexRuntime.DockerBaseImageURL
		}
		shellCommands = pc.CortexRuntime.ShellCommands
		preBuildCommands = pc.CortexRuntime.PreBuildCommands
		useUv = pc.CortexRuntime.UseUv
	}

	// Fallback to deprecated deployment fields if runtime fields not set
	if pc.CortexRuntime == nil || pc.CortexRuntime.PythonVersion == "" {
		if pc.Deployment.PythonVersion != "" {
			pythonVersion = pc.Deployment.PythonVersion
		}
	}
	if pc.CortexRuntime == nil || pc.CortexRuntime.DockerBaseImageURL == "" {
		if pc.Deployment.DockerBaseImageURL != "" {
			baseImage = pc.Deployment.DockerBaseImageURL
		}
	}
	if pc.CortexRuntime == nil || len(pc.CortexRuntime.ShellCommands) == 0 {
		if len(pc.Deployment.ShellCommands) > 0 {
			shellCommands = pc.Deployment.ShellCommands
		}
	}
	if pc.CortexRuntime == nil || len(pc.CortexRuntime.PreBuildCommands) == 0 {
		if len(pc.Deployment.PreBuildCommands) > 0 {
			preBuildCommands = pc.Deployment.PreBuildCommands
		}
	}
	if (pc.CortexRuntime == nil || pc.CortexRuntime.UseUv == nil) && pc.Deployment.UseUv != nil {
		useUv = pc.Deployment.UseUv
	}

	payload["pythonVersion"] = pythonVersion
	payload["baseImage"] = baseImage
	payload["shellCommands"] = shellCommands
	payload["preBuildCommands"] = preBuildCommands
	if useUv != nil {
		payload["useUv"] = *useUv
	}
}

// addPythonRuntimePayload adds python runtime fields to the payload
func (pc *ProjectConfig) addPythonRuntimePayload(payload map[string]any) {
	payload["runtime"] = "custom"

	pythonVersion := DefaultPythonVersion
	baseImage := DefaultDockerBaseImageURL
	var shellCommands []string
	var preBuildCommands []string
	var useUv *bool
	entrypoint := DefaultEntrypoint
	port := DefaultPort

	if pc.PythonRuntime != nil {
		if pc.PythonRuntime.PythonVersion != "" {
			pythonVersion = pc.PythonRuntime.PythonVersion
		}
		if pc.PythonRuntime.DockerBaseImageURL != "" {
			baseImage = pc.PythonRuntime.DockerBaseImageURL
		}
		shellCommands = pc.PythonRuntime.ShellCommands
		preBuildCommands = pc.PythonRuntime.PreBuildCommands
		useUv = pc.PythonRuntime.UseUv
		if len(pc.PythonRuntime.Entrypoint) > 0 {
			entrypoint = pc.PythonRuntime.Entrypoint
		}
		if pc.PythonRuntime.Port != 0 {
			port = pc.PythonRuntime.Port
		}
		payload["healthcheckEndpoint"] = pc.PythonRuntime.HealthcheckEndpoint
		payload["readycheckEndpoint"] = pc.PythonRuntime.ReadycheckEndpoint
	}

	// Fallback to deprecated deployment fields
	if pc.PythonRuntime == nil || pc.PythonRuntime.PythonVersion == "" {
		if pc.Deployment.PythonVersion != "" {
			pythonVersion = pc.Deployment.PythonVersion
		}
	}
	if pc.PythonRuntime == nil || pc.PythonRuntime.DockerBaseImageURL == "" {
		if pc.Deployment.DockerBaseImageURL != "" {
			baseImage = pc.Deployment.DockerBaseImageURL
		}
	}
	if pc.PythonRuntime == nil || len(pc.PythonRuntime.ShellCommands) == 0 {
		if len(pc.Deployment.ShellCommands) > 0 {
			shellCommands = pc.Deployment.ShellCommands
		}
	}
	if pc.PythonRuntime == nil || len(pc.PythonRuntime.PreBuildCommands) == 0 {
		if len(pc.Deployment.PreBuildCommands) > 0 {
			preBuildCommands = pc.Deployment.PreBuildCommands
		}
	}
	if (pc.PythonRuntime == nil || pc.PythonRuntime.UseUv == nil) && pc.Deployment.UseUv != nil {
		useUv = pc.Deployment.UseUv
	}

	payload["pythonVersion"] = pythonVersion
	payload["baseImage"] = baseImage
	payload["shellCommands"] = shellCommands
	payload["preBuildCommands"] = preBuildCommands
	if useUv != nil {
		payload["useUv"] = *useUv
	}
	payload["entrypoint"] = entrypoint
	payload["port"] = port
}

// addDockerRuntimePayload adds docker runtime fields to the payload
func (pc *ProjectConfig) addDockerRuntimePayload(payload map[string]any) {
	payload["runtime"] = "custom"

	if pc.DockerRuntime != nil {
		payload["dockerfilePath"] = pc.DockerRuntime.DockerfilePath
		payload["entrypoint"] = pc.DockerRuntime.Entrypoint
		port := DefaultPort
		if pc.DockerRuntime.Port != 0 {
			port = pc.DockerRuntime.Port
		}
		payload["port"] = port
		payload["healthcheckEndpoint"] = pc.DockerRuntime.HealthcheckEndpoint
		payload["readycheckEndpoint"] = pc.DockerRuntime.ReadycheckEndpoint
	}
}

// addCustomRuntimePayload adds deprecated custom runtime fields to the payload
func (pc *ProjectConfig) addCustomRuntimePayload(payload map[string]any) {
	// Handle deprecated [cerebrium.runtime.custom] section
	payload["runtime"] = "custom"

	// Also include build params from deployment (deprecated fallback)
	pythonVersion := DefaultPythonVersion
	baseImage := DefaultDockerBaseImageURL

	if pc.Deployment.PythonVersion != "" {
		pythonVersion = pc.Deployment.PythonVersion
	}
	if pc.Deployment.DockerBaseImageURL != "" {
		baseImage = pc.Deployment.DockerBaseImageURL
	}

	payload["pythonVersion"] = pythonVersion
	payload["baseImage"] = baseImage
	payload["shellCommands"] = pc.Deployment.ShellCommands
	payload["preBuildCommands"] = pc.Deployment.PreBuildCommands
	if pc.Deployment.UseUv != nil {
		payload["useUv"] = *pc.Deployment.UseUv
	}

	if pc.CustomRuntime != nil {
		payload["entrypoint"] = pc.CustomRuntime.Entrypoint
		payload["port"] = pc.CustomRuntime.Port
		payload["healthcheckEndpoint"] = pc.CustomRuntime.HealthcheckEndpoint
		payload["readycheckEndpoint"] = pc.CustomRuntime.ReadycheckEndpoint
		if pc.CustomRuntime.DockerfilePath != "" {
			payload["dockerfilePath"] = pc.CustomRuntime.DockerfilePath
		}
	}
}

// addPartnerRuntimePayload adds partner service fields to the payload
func (pc *ProjectConfig) addPartnerRuntimePayload(payload map[string]any) {
	if pc.PartnerService == nil {
		return
	}

	payload["partnerService"] = pc.PartnerService.Name
	payload["runtime"] = pc.PartnerService.Name
	if pc.PartnerService.Port != nil {
		payload["port"] = *pc.PartnerService.Port
	}
	if pc.PartnerService.ModelName != nil {
		payload["modelName"] = *pc.PartnerService.ModelName
	}
	if pc.PartnerService.Language != nil {
		payload["language"] = *pc.PartnerService.Language
	}
}
