#!/usr/bin/env python
"""B08 offline eval runner: exercises the adapter + grounding contract over
fixed fixtures (execution plan B08 step ⑥). No network, no provider key.

Fixture layout (evals/fixtures/b08/*.json):
    {"name": ..., "payload": {...AuditJobPayload...}, "expect": {...}}

expect keys:
    outcome            "success" | "failure"  (adapter outcome)
    min_assessments    int (success only)
    require_evidence   bool — every non-UNKNOWN assessment must cite a
                       manifest evidence id (grounding mirror of Go B07)
    forbid_unknown_rule bool — assessments must only use applicable rules

Run:  python evals/runners/run_b08_fixtures.py
Exit: 0 all fixtures pass; 1 otherwise. CI-safe (no model key needed).
"""

from __future__ import annotations

import asyncio
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "services" / "ai"))

from providers.base import Budget, FakeModel, StepFunVisionModel  # noqa: E402


def check_grounding(assessment: dict, payload: dict) -> list[str]:
    """Mirror of the Go grounding checks (B07): the eval runner re-validates
    adapter output the same way the worker does before persistence."""
    problems: list[str] = []
    manifest_ids = {e["evidence_id"] for e in payload.get("evidence", [])}
    rule_ids = {r["code"] for r in payload.get("applicable_rules", [])}

    rule_id = assessment.get("rule_id")
    status = assessment.get("status")
    if rule_id not in rule_ids:
        problems.append(f"unknown rule {rule_id}")
    if status not in ("PASS", "WARN", "FAIL", "UNKNOWN"):
        problems.append(f"invalid status {status}")
    if assessment.get("confidence", -1) < 0 or assessment.get("confidence", -1) > 1:
        problems.append("confidence out of range")
    refs = assessment.get("evidence_refs", [])
    if status != "UNKNOWN" and not refs:
        problems.append("non-UNKNOWN without evidence")
    for ref in refs:
        if ref.get("evidence_id") not in manifest_ids:
            problems.append(f"fabricated reference {ref.get('evidence_id')}")
    return problems


async def run_fixture(path: Path) -> tuple[str, bool, str]:
    fixture = json.loads(path.read_text(encoding="utf-8"))
    fixture.setdefault("name", path.stem)
    payload, expect = fixture["payload"], fixture["expect"]

    if fixture.get("adapter") == "stepfun-no-auth":
        # Provider-failure fixture: an adapter with an empty key must produce
        # a structured failure without any network call. Key access happens
        # lazily, so construct with an explicit empty key to force it.
        model = StepFunVisionModel(api_key="", base_url="http://127.0.0.1:1",
                                   max_requests=payload.get("budget", {}).get("max_provider_requests", 6))
    else:
        model = FakeModel(max_requests=payload.get("budget", {}).get("max_provider_requests", 6))

    try:
        response = await model.assess(payload)
    except Exception as exc:  # noqa: BLE001 - the runner reports, never crashes
        return fixture["name"], False, f"adapter raised: {exc}"

    outcome = response.get("outcome")
    if outcome != expect.get("outcome"):
        return fixture["name"], False, f"outcome {outcome} != expected {expect.get('outcome')}"

    detail = ""
    if outcome == "success":
        assessments = response.get("assessments", [])
        if len(assessments) < expect.get("min_assessments", 0):
            return fixture["name"], False, f"only {len(assessments)} assessments"
        problems: list[str] = []
        if expect.get("forbid_unknown_rule") or expect.get("require_evidence"):
            for a in assessments:
                problems.extend(check_grounding(a, payload))
        if expect.get("expected_grounding_failure"):
            # The fixture deliberately produces invalid assessments; the pass
            # condition is that grounding CATCHES them.
            if not problems:
                return fixture["name"], False, "grounding failed to catch invalid assessments"
            detail = f"grounding caught {len(problems)} problem(s): {problems[0]}"
            return fixture["name"], True, detail
        if problems:
            return fixture["name"], False, "; ".join(problems)
        detail = f"{len(assessments)} assessments grounded"
    else:
        error = response.get("error", {})
        detail = f"{error.get('code')} retryable={error.get('retryable')}"
        if expect.get("non_retryable") and error.get("retryable"):
            return fixture["name"], False, "expected non-retryable failure"
    return fixture["name"], True, detail


async def main() -> int:
    fixtures_dir = ROOT / "evals" / "fixtures" / "b08"
    ok = True
    for path in sorted(fixtures_dir.glob("*.json")):
        name, passed, detail = await run_fixture(path)
        print(f"{'PASS' if passed else 'FAIL'}  {name}: {detail}")
        ok = ok and passed
    print("eval runner:", "ALL PASS" if ok else "FAILURES")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(asyncio.run(main()))
