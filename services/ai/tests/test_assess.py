"""Tests for the internal assessment endpoint (B07 contract: always a
well-formed ProviderResponse with HTTP 200, even for malformed input)."""

from fastapi.testclient import TestClient

from app.main import app

VALID_PAYLOAD = {
    "job_id": "01912345-6789-7abc-9def-0123456789ad",
    "run_id": "01912345-6789-7abc-9def-0123456789ae",
    "attempt_token": "01912345-6789-7abc-9def-0123456789af",
    "idempotency_key": "run-x/assessment/attempt-1",
    "project": {
        "project_id": "01912345-6789-7abc-9def-0123456789ac",
        "entity_type": "restaurant",
        "target_locale": "en-US",
    },
    "standard": {
        "code": "IRRS",
        "version": "0.1.0",
        "rules_sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    },
    "evidence": [{"evidence_id": "01912345-6789-7abc-9def-0123456789ab"}],
    "applicable_rules": [{"code": "IRRS-D4-001"}],
    "budget": {"max_provider_requests": 6, "timeout_ms": 120000},
}


def test_assess_returns_structured_failure_for_valid_payload() -> None:
    resp = TestClient(app).post("/internal/assess", json=VALID_PAYLOAD)
    assert resp.status_code == 200
    body = resp.json()
    assert body["outcome"] == "failure"
    # The echo proves the worker's token guard can match this attempt.
    assert body["metadata"]["attempt_token"] == VALID_PAYLOAD["attempt_token"]
    assert body["metadata"]["job_id"] == VALID_PAYLOAD["job_id"]
    assert (
        body["error"]["retryable"] is False
    )  # B08 is a definitive "not yet", not a transient error


def test_assess_survives_garbage_input() -> None:
    resp = TestClient(app).post(
        "/internal/assess", content=b"not-json-at-all", headers={"Content-Type": "application/json"}
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["outcome"] == "failure"
    assert "unreadable" in body["error"]["message"]
