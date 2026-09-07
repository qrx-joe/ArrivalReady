"""Provider adapter tests — offline via httpx.MockTransport (B08).

Covers the safe-failure contract: auth/quota errors are definitive
non-retryable failures, success parses JSON output, and the shared budget
is enforced across calls.
"""

import asyncio
import json

import httpx
import pytest

from providers.base import Budget, BudgetExhausted, FakeModel, StepFunVisionModel

PAYLOAD = {
    "job_id": "j-1",
    "run_id": "r-1",
    "attempt_token": "t-1",
    "prompt_version": "assessment/v1",
    "project": {"project_id": "p", "entity_type": "restaurant", "target_locale": "en-US"},
    "evidence": [
        {
            "evidence_id": "e-1",
            "type": "image",
            "journey_stage": "act",
            "content_url": "https://example.test/signed",
        }
    ],
    "applicable_rules": [
        {
            "code": "IRRS-D4-001",
            "dimension": "D4",
            "description": "QR ordering language reach",
            "review_required": False,
        }
    ],
    "budget": {"max_provider_requests": 6, "timeout_ms": 1000},
}

CHAT_OK = {
    "choices": [
        {
            "message": {
                "content": json.dumps(
                    {
                        "assessments": [
                            {
                                "rule_id": "IRRS-D4-001",
                                "status": "FAIL",
                                "severity": "S1",
                                "evidence_refs": [
                                    {"evidence_id": "e-1", "locator": {"type": "full"}}
                                ],
                                "confidence": 0.9,
                                "needs_human_review": False,
                            }
                        ]
                    }
                )
            }
        }
    ],
    "usage": {"prompt_tokens": 10, "completion_tokens": 5},
}


def run(coro):  # small helper: keep tests sync without pytest-asyncio config
    return asyncio.run(coro)


def make_adapter(handler, api_key="k-test", max_requests=6) -> StepFunVisionModel:
    transport = httpx.MockTransport(handler)
    return StepFunVisionModel(
        api_key=api_key,
        base_url="https://mock.test/v1",
        model_id="step-1v-8k",
        max_requests=max_requests,
        http=httpx.AsyncClient(transport=transport),
    )


def test_fake_model_success_is_grounded() -> None:
    response = run(FakeModel().assess(PAYLOAD))
    assert response["outcome"] == "success"
    for a in response["assessments"]:
        if a["status"] != "UNKNOWN":
            assert a["evidence_refs"][0]["evidence_id"] == "e-1"


def test_fake_budget_is_shared_and_enforced() -> None:
    model = FakeModel(max_requests=1)
    run(model.assess(PAYLOAD))
    with pytest.raises(BudgetExhausted):
        run(model.assess(PAYLOAD))
    assert Budget(2).spent == 0  # fresh budget untouched


def test_stepfun_missing_key_is_definitive_auth_failure() -> None:
    response = run(
        make_adapter(
            lambda request: pytest.fail("network call must not happen"), api_key=""
        ).assess(PAYLOAD)
    )
    assert response["outcome"] == "failure"
    assert response["error"] == {
        "code": "provider_auth",
        "retryable": False,
        "message": "missing MODEL_API_KEY",
    }


def test_stepfun_auth_rejection_is_non_retryable() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["Authorization"] == "Bearer k-test"
        return httpx.Response(401, json={"error": "bad key"})

    response = run(make_adapter(handler).assess(PAYLOAD))
    assert response["outcome"] == "failure"
    assert response["error"]["code"] == "provider_auth"
    assert response["error"]["retryable"] is False


def test_stepfun_quota_is_non_retryable() -> None:
    response = run(make_adapter(lambda request: httpx.Response(429, json={})).assess(PAYLOAD))
    assert response["error"]["code"] == "provider_quota"
    assert response["error"]["retryable"] is False


def test_stepfun_success_parses_assessments() -> None:
    response = run(make_adapter(lambda request: httpx.Response(200, json=CHAT_OK)).assess(PAYLOAD))
    assert response["outcome"] == "success"
    assert response["metadata"]["provider"] == "stepfun"
    assert response["assessments"][0]["rule_id"] == "IRRS-D4-001"
    attempts = response["attempts"]
    assert attempts and attempts[0]["kind"] == "initial"
    assert attempts[0]["cost"] == {"known": False}  # unknown cost stays unknown, never 0


def test_stepfun_sends_image_content_url() -> None:
    seen: dict = {}

    def handler(request: httpx.Request) -> httpx.Response:
        seen["body"] = json.loads(request.content)
        return httpx.Response(200, json=CHAT_OK)

    run(make_adapter(handler).assess(PAYLOAD))
    content = seen["body"]["messages"][0]["content"]
    image_parts = [c for c in content if c["type"] == "image_url"]
    assert image_parts and image_parts[0]["image_url"]["url"] == "https://example.test/signed"
