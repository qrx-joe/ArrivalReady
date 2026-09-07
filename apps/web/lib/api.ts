/**
 * Typed fetch client for the Arrival Ready API (contracts/openapi is the
 * contract). Auth: Bearer token from localStorage (dev test identity);
 * 401 clears it so the next navigation lands on /login.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";
const TOKEN_KEY = "arrivalready.token";

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return window.localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  window.localStorage.setItem(TOKEN_KEY, token);
}

export function clearToken(): void {
  window.localStorage.removeItem(TOKEN_KEY);
}

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  const token = getToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const resp = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (resp.status === 401) {
    clearToken();
    throw new ApiError(401, "登录已失效，请重新登录");
  }
  const text = await resp.text();
  const body = text ? JSON.parse(text) : {};
  if (!resp.ok) {
    throw new ApiError(resp.status, body?.detail ?? body?.title ?? `HTTP ${resp.status}`);
  }
  return (body?.data ?? null) as T;
}

export const api = {
  loginDev: (email: string) =>
    fetch(`${API_BASE}/internal/dev-token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email }),
    }).then(async (r) => {
      if (!r.ok) throw new ApiError(r.status, "dev-token failed");
      return (await r.json()) as { token: string; email: string };
    }),

  createProject: (input: { name: string; entity_type: string; target_locale: string }) =>
    request<{ id: string; name: string }>("/projects", {
      method: "POST",
      body: JSON.stringify(input),
    }),
  listProjects: () =>
    request<{ id: string; name: string; entity_type: string; created_at: string }[]>("/projects"),
  getProject: (id: string) =>
    request<{
      id: string;
      name: string;
      entity_type: string;
      target_locale: string;
      status: string;
    }>(`/projects/${id}`),

  createUploadURL: (
    projectID: string,
    input: {
      evidence_type: string;
      mime_type: string;
      size_bytes: number;
      journey_stage: string;
    },
  ) =>
    request<{ evidence_id: string; upload_url: string; object_key: string }>(
      `/projects/${projectID}/evidence/upload-url`,
      { method: "POST", body: JSON.stringify(input) },
    ),
  completeUpload: (
    projectID: string,
    input: {
      evidence_id: string;
      object_key: string;
      sha256: string;
      idempotencyKey: string;
    },
  ) =>
    request<{ id: string; processing_status: string }>(`/projects/${projectID}/evidence`, {
      method: "POST",
      headers: { "Idempotency-Key": input.idempotencyKey },
      body: JSON.stringify({
        evidence_id: input.evidence_id,
        object_key: input.object_key,
        sha256: input.sha256,
      }),
    }),
  listEvidence: (projectID: string) =>
    request<
      { id: string; type: string; processing_status: string; sha256: string; created_at: string }[]
    >(`/projects/${projectID}/evidence`),

  createAudit: (projectID: string, evidence_ids: string[]) =>
    request<{ id: string; status: string }>(`/projects/${projectID}/audits`, {
      method: "POST",
      headers: { "Idempotency-Key": crypto.randomUUID() },
      body: JSON.stringify({ evidence_ids }),
    }),
  listAudits: (projectID: string) =>
    request<{ id: string; status: string; status_reason: string | null; created_at: string }[]>(
      `/projects/${projectID}/audits`,
    ),
  getAudit: (auditID: string) =>
    request<{
      run: {
        id: string;
        status: string;
        status_reason: string | null;
        parent_run_id?: string | null;
        standard: { code: string; version: string };
        scoring: ScoreReport | null;
        created_at: string;
      };
      findings: FindingSummary[];
    }>(`/audits/${auditID}`),

  submitReview: (
    findingID: string,
    body: {
      decision: string;
      note?: string;
      edits?: { assessment_status?: string };
      na_reason?: string;
    },
  ) =>
    request<FindingSummary>(`/findings/${findingID}/reviews`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  finalizeAudit: (auditID: string) =>
    request<ScoreReport>(`/audits/${auditID}/finalize`, {
      method: "POST",
      body: JSON.stringify({}),
    }),
  updateTask: (
    findingID: string,
    body: { workflow_status: string; version: number; reason?: string },
  ) =>
    request<{ id: string; workflow_status: string; version: number }>(
      `/findings/${findingID}/task`,
      {
        method: "PATCH",
        body: JSON.stringify(body),
      },
    ),
  retest: (auditID: string) =>
    request<{ id: string }>(`/audits/${auditID}/retest`, {
      method: "POST",
      headers: { "Idempotency-Key": crypto.randomUUID() },
      body: JSON.stringify({}),
    }),
  getDiff: (auditID: string) =>
    request<{
      entries: { rule_id: string; parent: string; child: string; comparable: boolean }[];
      note: string;
    }>(`/audits/${auditID}/diff`),

  getEvidence: (evidenceID: string) =>
    request<{
      id: string;
      type: string;
      processing_status: string;
      download_url: string | null;
      sha256: string;
    }>(`/evidence/${evidenceID}`),
  getFinding: (findingID: string) => request<FindingSummary>(`/findings/${findingID}`),
};

export type FindingSummary = {
  id: string;
  rule_id: string;
  assessment_status: "PASS" | "WARN" | "FAIL" | "UNKNOWN";
  severity: string;
  observation: string | null;
  reason: string | null;
  recommended_fix: string | null;
  confidence: number;
  review_status: string;
  original_candidate: unknown;
  evidence_refs: {
    evidence_id: string;
    locator: { type: string; bbox?: { x: number; y: number; w: number; h: number } };
  }[];
};

export type ScoreReport = {
  total_score: number | null;
  partial: boolean;
  coverage_pct: number;
  dimensions: {
    dimension: string;
    score: number;
    evaluated_weight?: number;
    applicable_weight?: number;
  }[];
  blocking: { rule_id: string; severity: string }[];
};
