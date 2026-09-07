"""Internal assessment endpoint (B07 mechanics; B08 fills the pipeline).

Contract (contracts/json-schema/job_payload + provider_response):
    Input:  AuditJobPayload posted by the Go worker.
    Output: a WELL-FORMED ProviderResponse with HTTP 200 — always, including
            for malformed input, so the worker's retry logic only ever sees
            structured outcomes.

B07 scope: structural validation only. The real assessment pipeline (provider
adapters, prompts, grounding support) lands in B08 and requires confirmed
model credentials (D-007). Until then the honest answer is a definitive,
non-retryable failure outcome — never a fabricated success, never a hang.
"""

import datetime
import hashlib

from fastapi import APIRouter, Request

router = APIRouter()


@router.post("/internal/assess")
async def assess(request: Request) -> dict[str, object]:
    raw = await request.body()

    job_id = "00000000-0000-0000-0000-000000000000"
    run_id = job_id
    attempt_token = job_id
    payload_note = "payload ok"
    try:
        payload = await request.json()
        job_id = payload["job_id"]
        run_id = payload["run_id"]
        attempt_token = payload["attempt_token"]
        if not payload.get("evidence") or not payload.get("applicable_rules"):
            payload_note = "payload missing evidence or applicable_rules"
    except Exception:
        payload_note = "payload unreadable"

    return {
        "outcome": "failure",
        "metadata": {
            "job_id": job_id,
            "run_id": run_id,
            "attempt_token": attempt_token,
            "provider": "none",
            "model": "not-configured",
            "model_version": None,
            "prompt_version": "assessment/v0",
            "schema_version": "provider-response-1.0",
            "input_hash": hashlib.sha256(raw).hexdigest(),
            "requested_at": datetime.datetime.now(datetime.UTC).isoformat(),
        },
        "error": {
            "code": "internal_error",
            "retryable": False,
            "message": (
                "assessment pipeline not implemented yet (lands in B08; "
                f"model credentials unconfirmed per D-007). payload note: {payload_note}"
            ),
        },
    }
