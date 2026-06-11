"""Configuration management using pydantic-settings."""

from typing import Optional
from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings loaded from environment variables."""

    model_config = SettingsConfigDict(
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
    )

    # Google Cloud / Vertex AI
    gcp_project_id: str = Field(..., description="GCP project ID for Vertex AI")
    gcp_region: str = Field(default="us-east5", description="GCP region for Vertex AI")
    google_application_credentials: Optional[str] = Field(
        default=None, description="Path to GCP service account JSON (uses gcloud default if not set)"
    )

    # GitHub
    github_token: str = Field(..., description="GitHub personal access token")

    # Temporal
    temporal_host: str = Field(default="localhost:7233", description="Temporal server host")
    temporal_namespace: str = Field(default="default", description="Temporal namespace")
    temporal_task_queue: str = Field(
        default="agentic-tasks", description="Temporal task queue name"
    )

    # API
    api_host: str = Field(default="0.0.0.0", description="API server host")
    api_port: int = Field(default=8000, description="API server port")
    log_level: str = Field(default="INFO", description="Logging level")

    # Workspace
    workspace_dir: str = Field(
        default="/tmp/agentic-workspaces",
        description="Directory for cloning repositories",
    )


# Global settings instance
settings = Settings()
