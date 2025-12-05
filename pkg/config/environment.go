package config

import (
	"fmt"
	"os"
)

// Environment represents the Cerebrium environment
type Environment string

const (
	EnvProd  Environment = "prod"
	EnvDev   Environment = "dev"
	EnvLocal Environment = "local"
)

// EnvConfig holds environment-specific URLs and settings
type EnvConfig struct {
	APIV1Url     string
	APIV2Url     string
	AuthUrl      string
	ClientID     string
	LogStreamUrl string
}

// GetEnvironment returns the current environment from CEREBRIUM_ENV
func GetEnvironment() Environment {
	env := os.Getenv("CEREBRIUM_ENV")
	if env == "" {
		return EnvProd
	}

	switch Environment(env) {
	case EnvProd, EnvDev, EnvLocal:
		return Environment(env)
	default:
		return EnvProd
	}
}

// GetEnvConfig returns the configuration for the specified environment
func GetEnvConfig(env Environment) (*EnvConfig, error) {
	switch env {
	case EnvProd:
		return &EnvConfig{
			APIV1Url:     getEnvOrDefault("REST_API_URL", "https://rest-api.cerebrium.ai"),
			APIV2Url:     getEnvOrDefault("REST_API_URL", "https://rest.cerebrium.ai"),
			AuthUrl:      getEnvOrDefault("AUTH_URL", "https://prod-cerebrium.auth.eu-west-1.amazoncognito.com/oauth2/token"),
			ClientID:     getEnvOrDefault("CLIENT_ID", "2om0uempl69t4c6fc70ujstsuk"),
			LogStreamUrl: getEnvOrDefault("LOGSTREAM_URL", "wss://logstream-api.aws.us-east-1.cerebrium.ai"),
		}, nil
	case EnvDev:
		return &EnvConfig{
			APIV1Url:     getEnvOrDefault("REST_API_URL", "https://dev-rest-api.cerebrium.ai"),
			APIV2Url:     getEnvOrDefault("REST_API_URL", "https://dev-rest.cerebrium.ai"),
			AuthUrl:      getEnvOrDefault("AUTH_URL", "https://dev-cerebrium.auth.eu-west-1.amazoncognito.com/oauth2/token"),
			ClientID:     getEnvOrDefault("CLIENT_ID", "207hg1caksrebuc79pcq1r3269"),
			LogStreamUrl: getEnvOrDefault("LOGSTREAM_URL", "wss://logstream-api.dev-aws.us-east-1.cerebrium.ai"),
		}, nil
	case EnvLocal:
		return &EnvConfig{
			APIV1Url:     getEnvOrDefault("REST_API_URL", "http://localhost:4100"),
			APIV2Url:     getEnvOrDefault("REST_API_URL", "http://localhost:4100"),
			AuthUrl:      getEnvOrDefault("AUTH_URL", "https://dev-cerebrium.auth.eu-west-1.amazoncognito.com/oauth2/token"),
			ClientID:     getEnvOrDefault("CLIENT_ID", "207hg1caksrebuc79pcq1r3269"),
			LogStreamUrl: getEnvOrDefault("LOGSTREAM_URL", "wss://logstream-api.dev-aws.us-east-1.cerebrium.ai"),
		}, nil
	default:
		return nil, fmt.Errorf("invalid environment: %s", env)
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
