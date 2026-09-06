"""Transport-level tests for the probe endpoints.

These run in CI without any provider key or network: a missing model config is
the EXPECTED state until B08, and the tests pin that behavior (503 + named
variables, never a fake 200).
"""

from fastapi.testclient import TestClient

from app.config import get_settings
from app.main import app


def _client() -> TestClient:
    return TestClient(app)


def test_healthz_always_ok(monkeypatch) -> None:
    monkeypatch.delenv("MODEL_API_KEY", raising=False)
    monkeypatch.delenv("MODEL_BASE_URL", raising=False)
    get_settings.cache_clear()
    resp = _client().get("/healthz")
    assert resp.status_code == 200
    assert resp.json() == {"status": "ok"}


def test_readyz_reports_missing_model_config(monkeypatch) -> None:
    monkeypatch.delenv("MODEL_API_KEY", raising=False)
    monkeypatch.delenv("MODEL_BASE_URL", raising=False)
    get_settings.cache_clear()
    resp = _client().get("/readyz")
    assert resp.status_code == 200  # probe endpoint reachable
    body = resp.json()
    assert body["status"] == "unavailable"
    assert "MODEL_API_KEY" in body["reason"]


def test_readyz_ready_when_configured(monkeypatch) -> None:
    monkeypatch.setenv("MODEL_API_KEY", "test-key")
    monkeypatch.setenv("MODEL_BASE_URL", "https://example.invalid")
    get_settings.cache_clear()
    resp = _client().get("/readyz")
    assert resp.json() == {"status": "ready"}
