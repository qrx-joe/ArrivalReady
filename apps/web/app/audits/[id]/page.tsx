"use client";

/**
 * Audit detail: polls run status while it is non-terminal (stops on terminal
 * or unmount — 执行方案 B09 步骤②) and lists candidate findings with their
 * human-review state. AI candidates are labelled 待人审 until reviewed.
 * Retest/diff (B12): a retest creates a NEW immutable run; diff compares
 * rule statuses against the parent and flags non-comparable cases.
 */

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { AppShell } from "@/components/layout/AppShell";
import { ApiError, api, FindingSummary, getToken } from "@/lib/api";

type ScoreReport = {
  total_score: number | null;
  partial: boolean;
  coverage_pct: number;
  dimensions: { dimension: string; score: number; applicable_weight?: number }[];
  blocking: { rule_id: string; severity: string }[];
};

const TERMINAL = new Set(["COMPLETED", "FAILED", "CANCELLED"]);
const STATUS_LABEL: Record<string, string> = {
  QUEUED: "排队中",
  INGESTING: "读取证据",
  ANALYZING: "AI 分析中",
  REVIEW_REQUIRED: "待人审",
  COMPLETED: "已完成",
  FAILED: "失败",
  CANCELLED: "已取消",
};

const RUN_BADGE: Record<string, string> = {
  QUEUED: "brand",
  INGESTING: "brand",
  ANALYZING: "brand",
  REVIEW_REQUIRED: "warning",
  COMPLETED: "success",
  FAILED: "danger",
  CANCELLED: "neutral",
};

const FINDING_TONE: Record<string, string> = {
  FAIL: "fail",
  WARN: "warn",
  PASS: "pass",
  UNKNOWN: "unknown",
};

function dimensionLevel(score: number): string {
  if (score < 50) return "level-fail";
  if (score < 70) return "level-warn";
  return "";
}

export default function AuditPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [run, setRun] = useState<AuditPageRun | null>(null);
  const [findings, setFindings] = useState<FindingSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [report, setReport] = useState<ScoreReport | null>(null);
  const [diff, setDiff] = useState<{
    entries: { rule_id: string; parent: string; child: string; comparable: boolean }[];
    note: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!getToken()) {
      window.location.href = "/login";
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const poll = async () => {
      try {
        const data = await api.getAudit(params.id);
        if (cancelled) return;
        setRun(data.run);
        setFindings(data.findings ?? []);
        if (!TERMINAL.has(data.run.status)) {
          timer = setTimeout(poll, 2000); // 轮询至终态；离页由 cancelled 停止
        }
      } catch (e) {
        if (!cancelled) setError(e instanceof ApiError ? e.message : "加载失败");
      }
    };
    poll();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
    };
  }, [params.id]);

  if (error) {
    return (
      <AppShell crumb="验收运行">
        <div className="alert danger" role="alert">
          <span>!</span>
          <span>{error}</span>
        </div>
      </AppShell>
    );
  }
  if (!run) {
    return (
      <AppShell crumb="验收运行">
        <div aria-busy="true" style={{ width: "min(680px, 100%)" }}>
          <div className="skeleton" style={{ width: "38%" }} />
          <div className="skeleton" />
          <div className="skeleton" style={{ width: "64%" }} />
          <div className="skeleton" style={{ width: "52%" }} />
        </div>
      </AppShell>
    );
  }

  return (
    <AppShell
      crumb={
        <>
          验收运行 / <b>{run.id.slice(0, 8)}…</b>
        </>
      }
      actions={
        <span className={`badge ${RUN_BADGE[run.status] ?? "neutral"}`}>
          {STATUS_LABEL[run.status] ?? run.status}
        </span>
      }
    >
      <div className="app-heading">
        <div>
          <h1>审计 {STATUS_LABEL[run.status] ?? run.status}</h1>
          <p>
            标准 {run.standard.code} {run.standard.version} ·{" "}
            {new Date(run.created_at).toLocaleString()}
            {run.parent_run_id && " · 复测运行"}
          </p>
        </div>
      </div>

      {run.status === "FAILED" && (
        <div className="alert danger" role="alert" style={{ marginBottom: "var(--s4)" }}>
          <span>!</span>
          <span>
            审计失败：{run.status_reason ?? "未知原因"}（原始证据已保留，可重新启动审计）
          </span>
        </div>
      )}
      {!TERMINAL.has(run.status) && (
        <div className="alert info" style={{ marginBottom: "var(--s4)" }}>
          <span>i</span>
          <span>AI 分析进行中，页面自动刷新…</span>
        </div>
      )}

      {run.status === "REVIEW_REQUIRED" && (
        <section className="panel" style={{ marginBottom: "var(--s5)" }}>
          <div className="panel-head">
            <h2>报告</h2>
            <span>确定性评分，基于已确认 Rule Status</span>
          </div>
          <div className="panel-body">
            <button
              className="btn btn-primary"
              onClick={async () => {
                try {
                  const r = await api.finalizeAudit(run.id);
                  setReport(r);
                } catch (e) {
                  setError(e instanceof ApiError ? e.message : "finalize 失败");
                }
              }}
            >
              生成报告（冻结评分）
            </button>
            {report && (
              <div style={{ marginTop: "var(--s4)" }}>
                <div className="score-block">
                  <div
                    className="score-ring"
                    style={{ "--score": report.total_score ?? 0 } as React.CSSProperties}
                  >
                    <div className="score-value">
                      <strong>{report.total_score ?? "—"}</strong>
                      <span>{report.total_score === null ? "Partial" : "Ready"}</span>
                    </div>
                  </div>
                  <div className="score-copy">
                    <h3>
                      {report.total_score === null
                        ? "部分评估（未出总分）"
                        : `总分 ${report.total_score} / 100`}
                    </h3>
                    <p>覆盖率 {report.coverage_pct}% · 分数由 Go 侧按标准版本确定性计算</p>
                  </div>
                </div>
                <div className="dimension-list" style={{ marginTop: "var(--s3)" }}>
                  {report.dimensions.map((d) => (
                    <div className="dimension" key={d.dimension}>
                      <span className="dimension-name">{d.dimension}</span>
                      <div className="bar">
                        <span
                          className={dimensionLevel(d.score)}
                          style={{ width: `${d.score}%` }}
                        />
                      </div>
                      <span className="dimension-score">{d.score}</span>
                    </div>
                  ))}
                </div>
                {report.blocking.length > 0 && (
                  <div className="alert danger" style={{ marginTop: "var(--s3)" }}>
                    <span>!</span>
                    <span>
                      阻断/关键问题：{report.blocking.map((b) => b.rule_id).join("、")}
                    </span>
                  </div>
                )}
              </div>
            )}
          </div>
        </section>
      )}

      {run.status === "COMPLETED" && (
        <section className="panel" style={{ marginBottom: "var(--s5)" }}>
          <div className="panel-head">
            <h2>复测（Before / After）</h2>
            <span>Retest 是新的不可变 run，旧报告保留</span>
          </div>
          <div className="panel-body">
            <div className="row">
              <button
                className="btn btn-primary"
                disabled={busy}
                onClick={async () => {
                  setBusy(true);
                  setError(null);
                  try {
                    const child = await api.retest(run.id);
                    router.push(`/audits/${child.id}`);
                  } catch (e) {
                    setError(e instanceof ApiError ? e.message : "复测创建失败");
                  } finally {
                    setBusy(false);
                  }
                }}
              >
                {busy ? "创建中…" : "创建复测（沿用本轮证据）"}
              </button>
              {run.parent_run_id && !diff && (
                <button
                  className="btn btn-secondary"
                  disabled={busy}
                  onClick={async () => {
                    setBusy(true);
                    try {
                      setDiff(await api.getDiff(run.id));
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  与上一轮对比
                </button>
              )}
            </div>
            {diff && (
              <div style={{ marginTop: "var(--s3)" }}>
                <table className="table">
                  <thead>
                    <tr>
                      <th>规则</th>
                      <th>上一轮</th>
                      <th>本轮</th>
                    </tr>
                  </thead>
                  <tbody>
                    {diff.entries.map((e) => (
                      <tr key={e.rule_id}>
                        <td className="mono">{e.rule_id}</td>
                        <td>{e.parent || "—"}</td>
                        <td>{e.child || "未评估"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {diff.note && (
                  <p className="input-hint" style={{ marginTop: "var(--s2)" }}>
                    {diff.note}
                  </p>
                )}
              </div>
            )}
          </div>
        </section>
      )}

      {TERMINAL.has(run.status) && findings.length === 0 && (
        <p className="muted">本次审计没有产出候选结论。</p>
      )}

      {findings.length > 0 && (
        <>
          <div className="app-heading" style={{ marginBottom: "var(--s3)" }}>
            <div>
              <h1 style={{ fontSize: "var(--text-lg)" }}>候选结论</h1>
              <p>{findings.length} 条 · AI 候选，待人审确认后才计入报告</p>
            </div>
          </div>
          <ul className="findings">
            {findings.map((f) => (
              <li key={f.id}>
                <Link className="finding-row" href={`/findings/${f.id}`}>
                  <span
                    className={`severity-dot ${FINDING_TONE[f.assessment_status] ?? "unknown"}`}
                  />
                  <span className="finding-main">
                    <b>
                      {f.assessment_status} · {f.rule_id}
                    </b>
                    <span>
                      {f.observation ? `观察：${f.observation}` : "—"} · 置信度 {f.confidence}
                    </span>
                  </span>
                  <span
                    className={`status-chip ${FINDING_TONE[f.assessment_status] ?? "unknown"}`}
                  >
                    {f.assessment_status}
                  </span>
                  <span
                    className={`badge ${f.review_status === "UNREVIEWED" ? "warning" : "success"}`}
                  >
                    {f.review_status === "UNREVIEWED" ? "待人审" : `已${f.review_status}`}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </AppShell>
  );
}

type AuditPageRun = {
  id: string;
  status: string;
  status_reason: string | null;
  parent_run_id?: string | null;
  standard: { code: string; version: string };
  created_at: string;
};
