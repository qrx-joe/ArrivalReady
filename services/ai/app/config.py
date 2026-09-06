"""AI service configuration.

Contract:
    Input:  environment variables (12-factor; compose injects them).
    Output: Settings instance; missing required-for-role values raise with a
            name-explicit error (B04 verify: 缺配置能明确报错).

The AI service only ever accepts internal calls from the Go API. At this stage
no provider credentials exist yet (D-007 unconfirmed), so absence of model keys
must NOT block boot — liveness stays truthful, readiness reports the gap.
"""

from functools import lru_cache

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=None, extra="ignore")

    env: str = Field(default="dev", pattern="^(dev|production)$")
    port: int = Field(default=8100, ge=1, le=65535)

    # Structured-output provider keys arrive with B08. Declared here so the
    # readiness probe can distinguish "not configured yet" from "broken".
    model_api_key: str | None = Field(default=None)
    model_base_url: str | None = Field(default=None)

    @property
    def model_configured(self) -> bool:
        return bool(self.model_api_key and self.model_base_url)


@lru_cache
def get_settings() -> Settings:
    return Settings()
