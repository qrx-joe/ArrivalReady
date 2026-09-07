"""AI service configuration.

Contract:
    Input:  environment variables, optionally layered from .env files so the
            PO can configure the model key without touching system settings:
              1. services/ai/.env   (service-local, gitignored)
              2. repo-root/.env     (shared local config, gitignored)
            Real environment variables win over .env values (12-factor);
            neither file is ever committed (docs/03 §4.3).
    Output: Settings instance; absence of model keys must NOT block boot —
            liveness stays truthful, readiness reports the gap (B04).
"""

from functools import lru_cache
from pathlib import Path

from pydantic import Field
from pydantic_settings import BaseSettings, SettingsConfigDict

_SERVICE_DIR = Path(__file__).resolve().parents[1]
_REPO_ROOT = Path(__file__).resolve().parents[3]

# Order matters: earlier files are overridden by later sources (real env wins
# over both files).
_ENV_FILES = (
    _SERVICE_DIR / ".env",
    _REPO_ROOT / ".env",
)


class Settings(BaseSettings):
    model_config = SettingsConfigDict(
        env_file=[str(p) for p in _ENV_FILES if p.exists()],
        extra="ignore",
    )

    env: str = Field(default="dev", pattern="^(dev|production)$")
    port: int = Field(default=8100, ge=1, le=65535)

    # Structured-output provider credentials (D-016: StepFun). Declared here
    # so the readiness probe can distinguish "not configured yet" from "broken".
    model_api_key: str | None = Field(default=None)
    model_base_url: str | None = Field(default=None)
    model_id: str | None = Field(default=None)

    # Offline adapter switch (tests / eval only; never with real data).
    arrival_fake_model: str | None = Field(default=None)

    @property
    def model_configured(self) -> bool:
        return bool(self.model_api_key and self.model_base_url)


@lru_cache
def get_settings() -> Settings:
    return Settings()
