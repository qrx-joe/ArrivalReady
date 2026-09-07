"""FastAPI transport edge for the AI service.

Liveness (/healthz) never touches configuration or network; readiness
(/readyz) reports the model-provider configuration gap explicitly instead of
answering a fake 200 — the same probe semantics as the Go API (B04).
"""

from fastapi import FastAPI

from app.assess import router as assess_router
from app.config import Settings, get_settings

app = FastAPI(title="Arrival Ready AI Service", version="0.1.0")
app.include_router(assess_router)


@app.get("/healthz")
def healthz() -> dict[str, str]:
    # Process truth only: no config access, no network.
    return {"status": "ok"}


@app.get("/readyz")
def readyz() -> dict[str, str]:
    settings: Settings = get_settings()
    if not settings.model_configured:
        # Honest not-configured state: MODEL_API_KEY/MODEL_BASE_URL arrive
        # with B08 (D-007 unconfirmed). Listing the exact variables makes the
        # gap actionable from the probe response alone.
        return {
            "status": "unavailable",
            "reason": "model provider not configured (MODEL_API_KEY / MODEL_BASE_URL empty)",
        }
    return {"status": "ready"}
