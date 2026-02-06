package projectconfig

import (
	"fmt"
	"os"
)

// Validate checks the configuration for errors and deprecated fields
func Validate(config *ProjectConfig) error {
	// Check required fields
	if config.Deployment.Name == "" {
		return fmt.Errorf("`deployment.name` is required in config file")
	}

	// Check for invalid provider
	if config.Hardware.Provider != nil && *config.Hardware.Provider == "coreweave" {
		return fmt.Errorf("cortex V4 does not support Coreweave. Please consider updating your app to AWS")
	}

	// Validate runtime configuration (CLI-level validation only)
	if err := validateRuntimeConfig(config); err != nil {
		return err
	}

	// Default gpu_count to 1 if compute is set but gpu_count is not
	if config.Hardware.Compute != nil && *config.Hardware.Compute != "CPU" {
		if config.Hardware.GPUCount == nil {
			defaultGPUCount := 1
			config.Hardware.GPUCount = &defaultGPUCount
		}
	}

	return nil
}

// validateRuntimeConfig validates the runtime configuration
// The CLI only validates things it needs locally - the backend validates runtime parameters
func validateRuntimeConfig(config *ProjectConfig) error {
	// Validate file paths that the CLI needs to check locally
	if config.Runtime != nil {
		// Check if dockerfile_path exists when specified
		dockerfilePath := config.Runtime.GetDockerfilePath()
		if dockerfilePath != "" {
			if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
				return fmt.Errorf("dockerfile not found at path: %s. Please ensure the path is correct", dockerfilePath)
			}
		}
	}

	// Check for main.py if required by runtime type
	if err := validateMainPyRequirement(config); err != nil {
		return err
	}

	return nil
}

// validateMainPyRequirement checks if main.py is required and exists
func validateMainPyRequirement(config *ProjectConfig) error {
	runtimeType := config.GetRuntimeType()

	// Docker runtime doesn't need main.py (uses Dockerfile)
	if config.Runtime != nil && config.Runtime.GetDockerfilePath() != "" {
		return nil
	}

	// Runtimes with custom entrypoint don't need main.py
	if config.Runtime != nil {
		entrypoint := config.Runtime.GetEntrypoint()
		if len(entrypoint) > 0 {
			return nil
		}
	}

	// Partner services and non-cortex runtimes don't need main.py
	// The backend validates what each runtime requires
	if runtimeType != "cortex" && runtimeType != "" {
		return nil
	}

	// Cortex runtime (default) requires main.py
	if _, err := os.Stat("main.py"); os.IsNotExist(err) {
		if _, err := os.Stat("./main.py"); os.IsNotExist(err) {
			return fmt.Errorf("main.py not found. Please ensure your project has a main.py file, or specify a custom runtime with a dockerfile or custom entrypoint")
		}
	}

	return nil
}
