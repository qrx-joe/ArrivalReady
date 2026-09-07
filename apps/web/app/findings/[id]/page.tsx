"use client";

/**
 * Evidence Viewer — the Magic Moment (PRD §12.4): every finding is human
 * verifiable in one click. The bounding box is drawn in ORIGINAL-image
 * normalized coordinates, so it stays anchored at any zoom or screen size
 * (定位随缩放保持). Rule, observation vs inference, confidence and the
 * 待人审 badge are all explicit (B09 步骤④).
 */

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/layout/AppShell";
import { ApiError, api, FindingSummary, getToken } from "@/lib/api";

type EvidenceImage = { id: string; url: string | null; status: string };

const STATUS_CHIP: Record<string, string> = {
  PASS: "pass",
  WARN: "warn",
  FAIL: "fail",
  UNKNOWN: "unknown",
};

const TASK_CHIP: Record<string, string> = {
  OPEN: "neutral",
  ACKNOWLEDGED: "brand",
  FIXING: "warning",
  READY_FOR_RETEST: "brand",
  ACCEPTED_RISK: "neutral",
  RESOLVED: "success",
  REOPENED: "danger",
};

const REVIEW_LABEL: Record<string, string> = {
  CONFIRMED: "已确认",
  EDITED: "已修订",
  REJECTED: "已驳回",
  NA: "不适用",
};

export default function FindingPage() {
  const params = useParams<{ id: string }>();
  const [finding, setFinding] = useState<FindingSummary | null>(null);
  const [images, setImages] = useState<EvidenceImage[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submitReview = useCallback(
    async (
      decision: string,
      edits?: { assessment_status?: string },
      naReason?: string,
      note?: string,
    ) => {
      if (!finding) return;
      setBusy(true);
      setReviewError(null);
      try {
        const updated = (await api.submitReview(finding.id, {
          decision,
          edits,
          na_reason: naReason,
          note,
        })) as FindingSummary;
        setFinding(updated);
      } catch (e) {
        setReviewError(e instanceof ApiError ? e.message : "提交失败");
      } finally {
        setBusy(false);
      }
    },
    [finding],
  );

  const askReject = useCallback(async () => {
    if (!finding) return;
    const note = window.prompt("驳回必须写明理由（为什么该候选结论不成立）：");
    if (note === null) return;
    if (!note.trim()) {
      setReviewError("驳回必须填写理由");
      return;
    }
    await submitReview("reject", undefined, undefined, note);
  }, [finding, submitReview]);

  const askNa = useCallback(async () => {
    if (!finding) return;
    const reason = window.prompt("NA 必须留理由（为什么此项不适用）：");
    if (reason === null) return;
    if (!reason.trim()) {
      setReviewError("NA 必须填写理由");
      return;
    }
    await submitReview("na", undefined, reason);
  }, [finding, submitReview]);

  const askEdit = useCallback(async () => {
    if (!finding) return;
    const status = window.prompt(
      "修订结论（PASS / WARN / FAIL / UNKNOWN）：",
      finding.assessment_status,
    );
    if (status === null) return;
    if (!["PASS", "WARN", "FAIL", "UNKNOWN"].includes(status)) {
      setReviewError("结论必须是 PASS/WARN/FAIL/UNKNOWN");
      return;
    }
    await submitReview("edit", { assessment_status: status });
  }, [finding, submitReview]);

  useEffect(() => {
    if (!getToken()) {
      window.location.href = "/login";
      return;
    }
    api
      .getFinding(params.id)
      .then(async (f) => {
        setFinding(f);
        // Resolve a private download URL per cited evidence (READY only).
        const imgs = await Promise.all(
          f.evidence_refs.map(async (ref): Promise<EvidenceImage> => {
            try {
              const e = await api.getEvidence(ref.evidence_id);
              return { id: ref.evidence_id, url: e.download_url, status: e.processing_status };
            } catch {
              return { id: ref.evidence_id, url: null, status: "UNAVAILABLE" };
            }
          }),
        );
        setImages(imgs);
      })
      .catch((e: ApiError) => setError(e.message));
  }, [params.id]);

  if (error) {
    return (
      <AppShell crumb="Finding">
        <div className="narrow">
          <div className="alert danger" role="alert">
            <span>!</span>
            <span>{error}</span>
          </div>
        </div>
      </AppShell>
    );
  }
  if (!finding) {
    return (
      <AppShell crumb="Finding">
        <div className="narrow" aria-busy="true">
          <div className="skeleton" style={{ width: "45%" }} />
          <div className="skeleton" />
          <div className="skeleton" style={{ width: "72%" }} />
        </div>
      </AppShell>
    );
  }

  const chip = STATUS_CHIP[finding.assessment_status] ?? "unknown";
  const unreviewed = finding.review_status === "UNREVIEWED";

  return (
    <AppShell
      crumb={
        <>
          Finding / <b>{finding.rule_id}</b>
        </>
      }
    >
      <div className="narrow">
        <button className="back-link link-btn" type="button" onClick={() => history.back()}>
          ← 返回
        </button>

        <div className="row" style={{ marginTop: "var(--s2)", marginBottom: "var(--s4)" }}>
          <h1 style={{ fontSize: "var(--text-xl)", letterSpacing: "-.03em" }}>
            {finding.assessment_status} · {finding.rule_id}
          </h1>
          <span className={`status-chip ${chip}`}>{finding.assessment_status}</span>
        </div>

        <div className="row" style={{ marginBottom: "var(--s5)" }}>
          <span className="badge neutral">严重度 {finding.severity}</span>
          <span className="badge neutral">置信度 {finding.confidence}</span>
          {unreviewed ? (
            <span className="badge brand">待人审（AI 候选，非最终结论）</span>
          ) : (
            <span className="badge success">
              人审：{REVIEW_LABEL[finding.review_status] ?? finding.review_status}
            </span>
          )}
        </div>

        {finding.review_status !== "UNREVIEWED" && <TaskPanel findingId={finding.id} />}
        {unreviewed ? (
          <section className="panel" style={{ marginBottom: "var(--s4)" }}>
            <div className="panel-head">
              <h2>人审处置（AI Suggests, Human Owns）</h2>
            </div>
            <div className="panel-body">
              <div className="row">
                <button
                  className="btn btn-primary btn-sm"
                  disabled={busy}
                  onClick={() => submitReview("confirm")}
                >
                  确认
                </button>
                <button className="btn btn-secondary btn-sm" disabled={busy} onClick={askEdit}>
                  修订结论
                </button>
                <button className="btn btn-secondary btn-sm" disabled={busy} onClick={askReject}>
                  驳回
                </button>
                <button className="btn btn-secondary btn-sm" disabled={busy} onClick={askNa}>
                  标记 NA
                </button>
              </div>
              {reviewError && (
                <p
                  style={{
                    color: "var(--fail)",
                    fontSize: "var(--text-sm)",
                    marginTop: "var(--s2)",
                  }}
                >
                  {reviewError}
                </p>
              )}
              <p className="input-hint" style={{ marginTop: "var(--s2)" }}>
                确认=采纳候选；修订=给出人工结论（保留 AI 原始候选）；驳回≠通过（无替代结论时保留
                UNKNOWN）；NA 需留理由并缩小评分范围。
              </p>
            </div>
          </section>
        ) : (
          <div className="alert success" style={{ marginBottom: "var(--s4)" }}>
            <span>✓</span>
            <span>
              已处置：{REVIEW_LABEL[finding.review_status] ?? finding.review_status}
              （处置记录不可覆盖；更正产生新记录）
            </span>
          </div>
        )}

        {images.map((img, i) => {
          const bbox = finding.evidence_refs[i]?.locator.bbox;
          return (
            <figure key={img.id} className="evidence-figure">
              {img.url ? (
                <span style={{ position: "relative", display: "block" }}>
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={img.url} alt={`证据 ${i + 1}`} />
                  {bbox && (
                    <span
                      className="bbox"
                      style={{
                        left: `${bbox.x * 100}%`,
                        top: `${bbox.y * 100}%`,
                        width: `${bbox.w * 100}%`,
                        height: `${bbox.h * 100}%`,
                      }}
                      aria-label="证据定位区域"
                    />
                  )}
                </span>
              ) : (
                <p className="alert info">
                  <span>i</span>
                  <span>证据不可查看（状态：{img.status}）。hash 与元数据已保留。</span>
                </p>
              )}
            </figure>
          );
        })}

        <section className="panel">
          <div className="panel-head">
            <h2>观察与推断</h2>
            <span>先证据，后建议</span>
          </div>
          <div className="panel-body detail-section">
            <p>
              <span className="detail-label">观察（事实）：</span>
              {finding.observation ?? "—"}
            </p>
            <p>
              <span className="detail-label">推断（理由）：</span>
              {finding.reason ?? "—"}
            </p>
            {finding.recommended_fix && (
              <p>
                <span className="detail-label">建议整改：</span>
                {finding.recommended_fix}
              </p>
            )}
          </div>
        </section>

        <div className="alert warning" style={{ marginTop: "var(--s4)" }}>
          <span>!</span>
          <span>以上是 AI 候选结论；确认、修改或驳回后才计入报告（AI Suggests, Human Owns）。</span>
        </div>
      </div>
    </AppShell>
  );
}

type TaskPanelProps = { findingId: string };

const NEXT_TASK: Record<string, { to: string; label: string }[]> = {
  OPEN: [{ to: "ACKNOWLEDGED", label: "确认整改" }],
  ACKNOWLEDGED: [{ to: "FIXING", label: "开始整改" }],
  FIXING: [
    { to: "READY_FOR_RETEST", label: "已改好，申请复测" },
    { to: "ACCEPTED_RISK", label: "接受风险" },
  ],
  READY_FOR_RETEST: [{ to: "REOPENED", label: "复测未通过，重开" }],
  REOPENED: [{ to: "FIXING", label: "重新整改" }],
};

function TaskPanel({ findingId }: TaskPanelProps) {
  const [status, setStatus] = useState<string | null>(null);
  const [version, setVersion] = useState(1);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = useCallback(async () => {
    try {
      const f = (await api.getFinding(findingId)) as unknown as {
        task?: { workflow_status: string; version: number };
      };
      if (f.task) {
        setStatus(f.task.workflow_status);
        setVersion(f.task.version);
      }
    } catch {
      // task may not exist yet; created on first transition
    }
  }, [findingId]);

  useEffect(() => {
    load();
  }, [load]);

  const move = useCallback(
    async (to: string, _label?: string) => {
      setBusy(true);
      setError(null);
      let reason = "";
      if (to === "ACCEPTED_RISK") {
        reason = window.prompt("接受风险必须填写理由：") ?? "";
        if (!reason.trim()) {
          setError("ACCEPTED_RISK 必须填写理由");
          setBusy(false);
          return;
        }
      }
      try {
        const updated = await api.updateTask(findingId, {
          workflow_status: to,
          version,
          reason,
        });
        setStatus(updated.workflow_status);
        setVersion(updated.version);
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "迁移失败");
      } finally {
        setBusy(false);
      }
    },
    [findingId, version],
  );

  const actions = status ? (NEXT_TASK[status] ?? []) : [];
  return (
    <section className="panel" style={{ marginBottom: "var(--s4)" }}>
      <div className="panel-head">
        <h2>整改任务</h2>
        <span className="row">
          {status && <span className={`badge ${TASK_CHIP[status] ?? "neutral"}`}>{status}</span>}
          <span className="mono muted">v{version}</span>
        </span>
      </div>
      <div className="panel-body">
        {error && (
          <p
            style={{
              color: "var(--fail)",
              fontSize: "var(--text-sm)",
              marginBottom: "var(--s2)",
            }}
          >
            {error}
          </p>
        )}
        {actions.length > 0 && (
          <div className="row">
            {actions.map((a) => (
              <button
                key={a.to}
                className="btn btn-secondary btn-sm"
                disabled={busy}
                onClick={() => move(a.to, a.label)}
              >
                {a.label}
              </button>
            ))}
          </div>
        )}
        {status === "READY_FOR_RETEST" && (
          <p className="input-hint" style={{ marginTop: "var(--s2)" }}>
            置为 RESOLVED 需要复测运行中同一规则有人审确认的
            PASS——系统会校验，仅点击不产生「已解决」。
          </p>
        )}
      </div>
    </section>
  );
}
