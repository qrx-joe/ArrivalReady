"""Arrival Ready AI service (probabilistic edge, TECH_SPEC §2.3).

Boundary (execution plan §5.2): this service receives an AuditJobPayload
(frozen rules + evidence manifest) and returns a ProviderResponse envelope.
It has no database connection, no review/score/task write paths, and no
credentials beyond the model provider. Domain pipelines live in plain modules;
FastAPI stays at the transport edge only (TECH_SPEC §3.3).
"""
