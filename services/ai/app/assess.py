"""Internal assessment endpoint (B08: real provider wired, safe-failure kept).

Contract (contracts/json-schema/job_payload + provider_response):
    Input:  AuditJobPayload posted by the Go worker.
    Output: a WELL-FORMED ProviderResponse with HTTP 200 — always, including
            for malformed input, so the worker's retry logic only ever sees
            structured outcomes.

Adapter selection (D-016: StepFun is the chosen provider):
    - MODEL_API_KEY + MODEL_BASE_URL set  -> StepFun vision adapter (real calls
      consume the shared request budget; first real smoke is capped at 6).
    - ARRIVAL_FAKE_MODEL=1                -> deterministic offline adapter
      (tests / offline eval only; NEVER with real data).
    - otherwise                           -> honest definitive failure. B08
      grounding stays in Go; this service only produces candidates.
"""

import os
from typing import Any

from fastapi import APIRouter, Request

from app.config import get_settings
from providers.base import FakeModel, StepFunVisionModel, failure_response

router = APIRouter()


def build_model() -> tuple[object | None, str]:
    """Return (model, reason) following the adapter selection order above."""
    if os.environ.get("ARRIVAL_FAKE_MODEL") == "1":
        return FakeModel(max_requests=6), "fake"
    settings = get_settings()
    if settings.model_configured:
        return (
            StepFunVisionModel(
                api_key=settings.model_api_key,
                base_url=settings.model_base_url,
            ),
            "stepfun",
        )
    return None, (
        "no adapter: MODEL_API_KEY/MODEL_BASE_URL not configured "
        "(or ARRIVAL_FAKE_MODEL=1 for offline)"
    )


async def assess_payload(raw: bytes, payload: dict[str, Any] | None) -> dict[str, Any]:
    model, reason = build_model()
    if model is None or payload is None:
        source = payload or {}
        job_id = str(source.get("job_id", "00000000-0000-0000-0000-000000000000"))
        return failure_response(
            {
                "job_id": job_id,
                "run_id": str(source.get("run_id", job_id)),
                "attempt_token": str(source.get("attempt_token", job_id)),
                "prompt_version": "assessment/v1",
            },
            provider="none",
            model="not-configured",
            code="internal_error",
            retryable=False,
            message=f"assessment unavailable: {reason}",
        )
    result: dict[str, Any] = await model.assess(payload)  # type: ignore[attr-defined]
    return result


@router.post("/internal/assess")
async def assess(request: Request) -> dict[str, Any]:
    raw = await request.body()
    payload: dict[str, Any] | None = None
    try:
        parsed = await request.json()
        if isinstance(parsed, dict):
            payload = parsed
    except Exception:
        payload = None
    return await assess_payload(raw, payload)
