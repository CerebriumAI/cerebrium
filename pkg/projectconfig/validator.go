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

	// Validate runtime configuration
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
func validateRuntimeConfig(config *ProjectConfig) error {
	// Check for mutual exclusion of runtime types
	runtimeCount := 0
	var runtimeNames []string

	if config.CortexRuntime != nil {
		runtimeCount++
		runtimeNames = append(runtimeNames, "cortex")
	}
	if config.PythonRuntime != nil {
		runtimeCount++
		runtimeNames = append(runtimeNames, "python")
	}
	if config.DockerRuntime != nil {
		runtimeCount++
		runtimeNames = append(runtimeNames, "docker")
	}
	if config.CustomRuntime != nil {
		runtimeCount++
		runtimeNames = append(runtimeNames, "custom")
	}

	if runtimeCount > 1 {
		return fmt.Errorf("only one runtime type can be specified, found: %v", runtimeNames)
	}

	// Validate docker runtime
	if config.DockerRuntime != nil {
		if err := validateDockerRuntime(config.DockerRuntime); err != nil {
			return err
		}
	}

	// Validate deprecated custom runtime
	if config.CustomRuntime != nil {
		if err := validateCustomRuntime(config.CustomRuntime); err != nil {
			return err
		}
	}

	// Check for main.py if using cortex runtime (or no runtime specified)
	if err := validateMainPyRequirement(config); err != nil {
		return err
	}

	return nil
}

// validateDockerRuntime validates the docker runtime configuration
func validateDockerRuntime(runtime *DockerRuntimeConfig) error {
	// dockerfile_path is required for docker runtime
	if runtime.DockerfilePath == "" {
		return fmt.Errorf("[cerebrium.runtime.docker] requires `dockerfile_path` to be specified")
	}

	// Check if dockerfile exists
	if _, err := os.Stat(runtime.DockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("dockerfile not found at path: %s. Please ensure the path is correct", runtime.DockerfilePath)
	}

	return nil
}

// validateCustomRuntime validates the deprecated custom runtime configuration
func validateCustomRuntime(runtime *CustomRuntimeConfig) error {
	// Validate dockerfile path if specified
	if runtime.DockerfilePath != "" {
		if _, err := os.Stat(runtime.DockerfilePath); os.IsNotExist(err) {
			return fmt.Errorf("dockerfile not found at path: %s. Please ensure the path is correct", runtime.DockerfilePath)
		}
	}
	return nil
}

// validateMainPyRequirement checks if main.py is required and exists
func validateMainPyRequirement(config *ProjectConfig) error {
	// main.py is only required for cortex runtime (or when no runtime is specified)
	runtimeType := config.GetRuntimeType()

	// Docker runtime doesn't need main.py
	if runtimeType == RuntimeTypeDocker {
		return nil
	}

	// Partner services don't need main.py
	if runtimeType == RuntimeTypePartner {
		return nil
	}

	// Python runtime with custom entrypoint doesn't need main.py
	if runtimeType == RuntimeTypePython && config.PythonRuntime != nil {
		if len(config.PythonRuntime.Entrypoint) > 0 && config.PythonRuntime.Entrypoint[0] != "uvicorn" {
			return nil
		}
		// Python runtime is for custom ASGI apps, doesn't need main.py
		return nil
	}

	// Custom runtime with dockerfile or custom entrypoint doesn't need main.py
	if runtimeType == RuntimeTypeCustom && config.CustomRuntime != nil {
		if config.CustomRuntime.DockerfilePath != "" {
			return nil
		}
		if len(config.CustomRuntime.Entrypoint) > 0 && config.CustomRuntime.Entrypoint[0] != "uvicorn" {
			return nil
		}
	}

	// Cortex runtime (default) requires main.py
	if _, err := os.Stat("main.py"); os.IsNotExist(err) {
		if _, err := os.Stat("./main.py"); os.IsNotExist(err) {
			return fmt.Errorf("main.py not found. Please ensure your project has a main.py file, or specify a custom runtime with a dockerfile or custom entrypoint")
		}
	}

	return nil
}
