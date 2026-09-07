#!/usr/bin/env python
"""First real-provider smoke test (execution plan B08 verify, D-016 StepFun).

Budget discipline (execution plan §6): this script spends at most
    1 GET /models (read-only listing)  +  1 chat completion  (+1 repair)
against the provider. Auth/quota errors stop immediately.

The input image is a SELF-MADE fixture (engineering-slice allowance, plan §1),
clearly labelled "not a real shop" — this smoke proves the API path, request
shape and structured-output parsing, NOT assessment quality or business value.

Usage:  uv run python evals/runners/../scripts? -> services/ai/smoke_stepfun.py
        (run from services/ai:  uv run python smoke_stepfun.py <image.jpg>)
"""

from __future__ import annotations

import asyncio
import base64
import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import httpx

from app.config import get_settings
from providers.base import StepFunVisionModel


async def main(image_path: str) -> int:
    settings = get_settings()
    if not settings.model_configured:
        print("MODEL_API_KEY / MODEL_BASE_URL not configured")
        return 2
    model_id = settings.model_id or "step-1v-8k"
    raw = Path(image_path).read_bytes()
    b64 = base64.b64encode(raw).decode()
    data_url = f"data:image/jpeg;base64,{b64}"

    payload = {
        "job_id": "smoke-job", "run_id": "smoke-run", "attempt_token": "smoke-token",
        "prompt_version": "assessment/v1",
        "project": {"project_id": "smoke", "entity_type": "restaurant", "target_locale": "en-US"},
        "evidence": [{"evidence_id": "ev-smoke-1", "type": "image",
                      "journey_stage": "understand", "content_url": data_url}],
        "applicable_rules": [
            {"code": "IRRS-D2-001", "dimension": "D2",
             "description": "菜单对目标访客语言可达（目标语言版本或可理解翻译路径）",
             "review_required": False},
            {"code": "IRRS-D5-001", "dimension": "D5",
             "description": "支付方式以标识明示（国际卡/移动支付/现金）",
             "review_required": True},
        ],
        "budget": {"max_provider_requests": 6, "timeout_ms": 120000},
    }

    headers = {"Authorization": f"Bearer {settings.model_api_key}"}
    async with httpx.AsyncClient(timeout=120) as client:
        # Step 0 (read-only): confirm the chosen model id is available.
        listing = await client.get(f"{settings.model_base_url}/models", headers=headers)
        if listing.status_code != 200:
            print(f"models list failed: HTTP {listing.status_code}: {listing.text[:200]}")
            return 3
        available = [m["id"] for m in listing.json().get("data", [])]
        if model_id not in available:
            print(f"model {model_id} not in account's model list; pick one from: {available}")
            return 3
        print(f"[0/1] models list OK; using {model_id}")

        # Step 1 (the one real generation call): reuse the production adapter,
        # but hand it an httpx client bound to this script so we can print
        # usage. The adapter enforces the shared budget internally.
        adapter = StepFunVisionModel(
            api_key=settings.model_api_key,
            base_url=settings.model_base_url,
            model_id=model_id,
            max_requests=2,  # initial + at most one repair
        )
        response = await adapter.assess(payload)

    print("=== ProviderResponse ===")
    print(json.dumps(
        {"outcome": response.get("outcome"),
         "provider": response.get("metadata", {}).get("provider"),
         "model": response.get("metadata", {}).get("model"),
         "error": response.get("error")},
        ensure_ascii=False))
    if response.get("outcome") == "success":
        for a in response.get("assessments", []):
            refs = [r.get("evidence_id") for r in a.get("evidence_refs", [])]
            print(f"- {a.get('rule_id')}: {a.get('status')} "
                  f"severity={a.get('severity')} confidence={a.get('confidence')}")
            print(f"  refs: {refs}")
            print(f"  observation: {a.get('observation', '')[:120]}")
        attempts = response.get("attempts", [])
        if attempts:
            usage = attempts[0].get("usage")
            latency = attempts[0].get("latency_ms")
            cost = attempts[0].get("cost")
            print(f"usage: {usage} | latency_ms: {latency} | cost: {cost}")
        return 0
    print(json.dumps(response.get("error", {}), ensure_ascii=False))
    return 1


if __name__ == "__main__":
    image = sys.argv[1] if len(sys.argv) > 1 else "smoke_menu.jpg"
    sys.exit(asyncio.run(main(image)))
