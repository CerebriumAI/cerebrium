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

	// Validate dockerfile path if specified
	if config.CustomRuntime != nil && config.CustomRuntime.DockerfilePath != "" {
		if _, err := os.Stat(config.CustomRuntime.DockerfilePath); os.IsNotExist(err) {
			return fmt.Errorf("dockerfile not found at path: %s. Please ensure the path is correct", config.CustomRuntime.DockerfilePath)
		}
	}

	// Check for main.py if not using custom runtime with dockerfile or custom entrypoint
	hasDockerfile := config.CustomRuntime != nil && config.CustomRuntime.DockerfilePath != ""
	hasCustomEntrypoint := config.CustomRuntime != nil &&
		len(config.CustomRuntime.Entrypoint) > 0 &&
		config.CustomRuntime.Entrypoint[0] != "uvicorn"

	if !hasDockerfile && !hasCustomEntrypoint {
		// Check if main.py exists
		if _, err := os.Stat("main.py"); os.IsNotExist(err) {
			if _, err := os.Stat("./main.py"); os.IsNotExist(err) {
				return fmt.Errorf("main.py not found. Please ensure your project has a main.py file, or specify a custom runtime with a dockerfile or custom entrypoint")
			}
		}
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
