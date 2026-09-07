"""Test isolation: unit tests NEVER spend provider budget.

services/ai/.env holds the PO's real StepFun key. An autouse fixture forces
the offline fake adapter and clears the cached settings, so no test can make
a real provider call by accident. The real HTTP path is covered separately by
httpx.MockTransport tests (test_providers.py), which never touch get_settings.
"""

import pytest

from app.config import get_settings


@pytest.fixture(autouse=True)
def _force_offline(monkeypatch):
    monkeypatch.setenv("ARRIVAL_FAKE_MODEL", "1")
    monkeypatch.delenv("MODEL_API_KEY", raising=False)
    monkeypatch.delenv("MODEL_BASE_URL", raising=False)
    get_settings.cache_clear()
    yield
    get_settings.cache_clear()
