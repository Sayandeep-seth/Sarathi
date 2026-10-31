"""Runtime settings for the SARATHI AI service (environment-driven)."""

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_prefix="SARATHI_AI_", extra="ignore")

    env: str = Field(default="local", pattern="^(local|staging|production)$")
    version: str = "dev"
    # Shared secret presented by the Go backend as `X-Service-Token`.
    # When unset, every /internal route fails closed with 503.
    service_token: str | None = None
