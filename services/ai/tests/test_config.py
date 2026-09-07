"""Config loading: the PO configures the model key in services/ai/.env —
this test pins that file-based configuration actually reaches Settings
(regression for the env_file=None usability gap found in B08)."""

from pathlib import Path

from app.config import Settings


def test_settings_load_from_env_file(tmp_path: Path) -> None:
    env_file = tmp_path / ".env"
    env_file.write_text(
        "MODEL_API_KEY=sk-test-123\n"
        "MODEL_BASE_URL=https://api.stepfun.com/v1\n"
        "MODEL_ID=step-1v-8k\n",
        encoding="utf-8",
    )
    settings = Settings(_env_file=env_file)
    assert settings.model_api_key == "sk-test-123"
    assert settings.model_base_url == "https://api.stepfun.com/v1"
    assert settings.model_id == "step-1v-8k"
    assert settings.model_configured is True


def test_settings_defaults_without_env_file(tmp_path: Path) -> None:
    settings = Settings(_env_file=tmp_path / "nonexistent.env")
    assert settings.model_api_key is None
    assert settings.model_configured is False
