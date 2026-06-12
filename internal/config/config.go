package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	GCPProjectID                 string `env:"GCP_PROJECT_ID,required"`
	GCPRegion                    string `env:"GCP_REGION" envDefault:"us-east5"`
	GoogleApplicationCredentials string `env:"GOOGLE_APPLICATION_CREDENTIALS"`

	GitHubToken string `env:"GITHUB_TOKEN,required"`

	TemporalHost      string `env:"TEMPORAL_HOST" envDefault:"localhost:7233"`
	TemporalNamespace string `env:"TEMPORAL_NAMESPACE" envDefault:"default"`
	TemporalTaskQueue string `env:"TEMPORAL_TASK_QUEUE" envDefault:"agentic-tasks"`

	APIHost  string `env:"API_HOST" envDefault:"0.0.0.0"`
	APIPort  int    `env:"API_PORT" envDefault:"8000"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"INFO"`

	WorkspaceDir string `env:"WORKSPACE_DIR" envDefault:"/tmp/agentic-workspaces"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return cfg, nil
}
