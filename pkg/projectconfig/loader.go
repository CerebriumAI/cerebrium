package projectconfig

import (
	"fmt"
	"os"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// Default values for CLI-only file-packaging concerns.
// All payload defaults (pythonVersion, baseImage, provider, region, scaling, auth,
// entrypoint, port, healthcheck, etc.) are intentionally not set here — the backend
// applies them when fields are absent from the request.
var (
	DefaultInclude = []string{"./*", "main.py", "cerebrium.toml"}
	DefaultExclude = []string{".*"}
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

	// Parse hardware section.
	// The compute decode hook lets `compute` accept either a scalar or an array
	// without leaking that polymorphism into the rest of the codebase. Hardware
	// has no duration or comma-slice fields, so viper's default hooks aren't
	// needed here.
	if v.IsSet("cerebrium.hardware") {
		if err := v.UnmarshalKey("cerebrium.hardware", &config.Hardware,
			viper.DecodeHook(computeFieldDecodeHook()),
		); err != nil {
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

	// Parse container_runtime scalar from [cerebrium.runtime].
	// Coexists with [cerebrium.runtime.custom] / [cerebrium.runtime.<partner>] sub-tables.
	if v.IsSet("cerebrium.runtime.container_runtime") {
		cr := v.GetString("cerebrium.runtime.container_runtime")
		if cr != "v1" && cr != "v2" {
			return nil, fmt.Errorf("invalid container_runtime %q: must be \"v1\" or \"v2\"", cr)
		}
		config.ContainerRuntime = &cr
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

	// Validate compute_tier before applying defaults
	if config.Scaling.ComputeTier != nil {
		tier := *config.Scaling.ComputeTier
		if tier != "interruptible" && tier != "protected" {
			return nil, fmt.Errorf("invalid compute_tier %q: must be \"interruptible\" or \"protected\"", tier)
		}
	}

	// Apply defaults for missing fields
	applyDefaults(&config)

	return &config, nil
}

// computeFieldDecodeHook teaches mapstructure how to read the polymorphic
// `compute` field. Without it viper would refuse to decode a TOML scalar into
// the ComputeField slice type. Rejects empty strings so downstream code can
// rely on `IsSet() => Primary() != ""`.
func computeFieldDecodeHook() mapstructure.DecodeHookFunc {
	computeFieldType := reflect.TypeOf(ComputeField{})
	return func(_ reflect.Type, to reflect.Type, data any) (any, error) {
		if to != computeFieldType {
			return data, nil
		}
		switch v := data.(type) {
		case string:
			if v == "" {
				return nil, fmt.Errorf("compute must not be empty")
			}
			return ComputeField{v}, nil
		case []any:
			if len(v) == 0 {
				return nil, fmt.Errorf("compute array must not be empty")
			}
			out := make(ComputeField, len(v))
			for i, item := range v {
				s, ok := item.(string)
				if !ok || s == "" {
					return nil, fmt.Errorf("compute values must be non-empty strings")
				}
				out[i] = s
			}
			return out, nil
		default:
			return nil, fmt.Errorf("compute must be a string or array of strings")
		}
	}
}

// applyDefaults sets default values for CLI-only fields that weren't specified in the config.
// Payload fields (pythonVersion, baseImage, provider, region, scaling, auth, entrypoint,
// port, healthcheck, etc.) are deliberately left unset so the backend can apply its own defaults.
func applyDefaults(config *ProjectConfig) {
	// File-packaging defaults — these aren't sent to the backend; they drive the deploy zip.
	if len(config.Deployment.Include) == 0 {
		config.Deployment.Include = DefaultInclude
	}
	if len(config.Deployment.Exclude) == 0 {
		config.Deployment.Exclude = DefaultExclude
	}
}
