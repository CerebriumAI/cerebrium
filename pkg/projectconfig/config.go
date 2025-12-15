package projectconfig

// ProjectConfig represents the complete cerebrium.toml configuration
type ProjectConfig struct {
	Deployment     DeploymentConfig      `mapstructure:"deployment" toml:"deployment"`
	Hardware       HardwareConfig        `mapstructure:"hardware" toml:"hardware"`
	Scaling        ScalingConfig         `mapstructure:"scaling" toml:"scaling"`
	Dependencies   DependenciesConfig    `mapstructure:"dependencies" toml:"dependencies"`
	CustomRuntime  *CustomRuntimeConfig  `mapstructure:"custom" toml:"runtime,omitempty"`
	PartnerService *PartnerServiceConfig `mapstructure:"partner" toml:"partner,omitempty"`
}

// DeploymentConfig represents the [cerebrium.deployment] section
type DeploymentConfig struct {
	Name                            string   `mapstructure:"name" toml:"name,omitempty"`
	PythonVersion                   string   `mapstructure:"python_version" toml:"python_version,omitempty"`
	DockerBaseImageURL              string   `mapstructure:"docker_base_image_url" toml:"docker_base_image_url,omitempty"`
	Include                         []string `mapstructure:"include" toml:"include,omitempty"`
	Exclude                         []string `mapstructure:"exclude" toml:"exclude,omitempty"`
	ShellCommands                   []string `mapstructure:"shell_commands" toml:"shell_commands,omitempty"`
	PreBuildCommands                []string `mapstructure:"pre_build_commands" toml:"pre_build_commands,omitempty"`
	DisableAuth                     *bool    `mapstructure:"disable_auth" toml:"disable_auth,omitempty"`
	UseUv                           *bool    `mapstructure:"use_uv" toml:"use_uv,omitempty"`
	DeploymentInitializationTimeout *int     `mapstructure:"deployment_initialization_timeout" toml:"deployment_initialization_timeout,omitempty"`
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
	Name string `mapstructure:"name" toml:"name,omitempty"`
	Port *int   `mapstructure:"port" toml:"port,omitempty"`
}

// ToPayload converts the project config to an API payload
func (pc *ProjectConfig) ToPayload() map[string]any {
	// TODO(wes): json encode?
	payload := make(map[string]any)

	// Deployment fields
	payload["name"] = pc.Deployment.Name
	payload["pythonVersion"] = pc.Deployment.PythonVersion
	payload["baseImage"] = pc.Deployment.DockerBaseImageURL
	payload["include"] = pc.Deployment.Include
	payload["exclude"] = pc.Deployment.Exclude
	payload["shellCommands"] = pc.Deployment.ShellCommands
	payload["preBuildCommands"] = pc.Deployment.PreBuildCommands

	if pc.Deployment.DisableAuth != nil {
		payload["disableAuth"] = *pc.Deployment.DisableAuth
	}
	if pc.Deployment.UseUv != nil {
		payload["useUv"] = *pc.Deployment.UseUv
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

	// Runtime configuration
	if pc.CustomRuntime != nil && pc.PartnerService != nil {
		// Both custom runtime and partner service
		payload["entrypoint"] = pc.CustomRuntime.Entrypoint
		payload["port"] = pc.CustomRuntime.Port
		payload["healthcheckEndpoint"] = pc.CustomRuntime.HealthcheckEndpoint
		payload["readycheckEndpoint"] = pc.CustomRuntime.ReadycheckEndpoint
		payload["dockerfilePath"] = pc.CustomRuntime.DockerfilePath
		payload["partnerService"] = pc.PartnerService.Name
		payload["runtime"] = pc.PartnerService.Name
	} else if pc.CustomRuntime != nil {
		// Custom runtime only
		payload["entrypoint"] = pc.CustomRuntime.Entrypoint
		payload["port"] = pc.CustomRuntime.Port
		payload["healthcheckEndpoint"] = pc.CustomRuntime.HealthcheckEndpoint
		payload["readycheckEndpoint"] = pc.CustomRuntime.ReadycheckEndpoint
		payload["dockerfilePath"] = pc.CustomRuntime.DockerfilePath
		payload["runtime"] = "custom"
	} else if pc.PartnerService != nil {
		// Partner service only
		payload["partnerService"] = pc.PartnerService.Name
		payload["runtime"] = pc.PartnerService.Name
		if pc.PartnerService.Port != nil {
			payload["port"] = *pc.PartnerService.Port
		}
	} else {
		// Default cortex runtime
		payload["runtime"] = "cortex"
	}

	return payload
}
