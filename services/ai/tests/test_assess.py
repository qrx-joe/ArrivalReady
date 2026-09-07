"""Tests for the internal assessment endpoint (B07/B08 contract: always a
well-formed ProviderResponse with HTTP 200, even for malformed input).

conftest.py forces the offline adapter for the whole suite — unit tests must
never spend real provider budget. The no-adapter honest-failure path is
exercised explicitly by opting out inside that one test."""

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


def test_assess_offline_adapter_returns_success_for_valid_payload() -> None:
    # conftest forces ARRIVAL_FAKE_MODEL=1: the endpoint answers from the
    # deterministic offline adapter without any network.
    resp = TestClient(app).post("/internal/assess", json=VALID_PAYLOAD)
    assert resp.status_code == 200
    body = resp.json()
    assert body["outcome"] == "success"
    # The echo proves the worker's token guard can match this attempt.
    assert body["metadata"]["attempt_token"] == VALID_PAYLOAD["attempt_token"]
    assert body["metadata"]["job_id"] == VALID_PAYLOAD["job_id"]
    assert body["metadata"]["provider"] == "fake"


def test_assess_payload_honest_failure_when_model_none() -> None:
    # Direct unit test of the no-adapter branch (isolation from services/ai/.env,
    # which holds a real key on dev machines): the endpoint contract is a
    # definitive structured failure, never a fabricated success.
    import asyncio

    from app.assess import assess_payload

    body = asyncio.run(assess_payload(b"{}", {"job_id": "j-1", "run_id": "r-1",
                                              "attempt_token": "t-1"},
                                      model=None, model_reason="no adapter configured"))
    assert body["outcome"] == "failure"
    assert body["error"]["retryable"] is False
    assert "assessment unavailable" in body["error"]["message"]
    assert body["metadata"]["job_id"] == "j-1"  # echo for the worker's token guard


def test_assess_survives_garbage_input() -> None:
    resp = TestClient(app).post(
        "/internal/assess", content=b"not-json-at-all", headers={"Content-Type": "application/json"}
    )
    assert resp.status_code == 200
    body = resp.json()
    assert body["outcome"] == "failure"
    # malformed input must still yield a structured envelope
    assert "assessment unavailable" in body["error"]["message"]
