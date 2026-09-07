"use client";

/**
 * Audit detail: polls run status while it is non-terminal (stops on terminal
 * or unmount — 执行方案 B09 步骤②) and lists candidate findings with their
 * human-review state. AI candidates are labelled 待人审 until reviewed.
 */

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

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

const FINDING_STYLE: Record<string, string> = {
  FAIL: "bg-red-50 text-red-700 border-red-200",
  WARN: "bg-amber-50 text-amber-700 border-amber-200",
  PASS: "bg-green-50 text-green-700 border-green-200",
  UNKNOWN: "bg-gray-50 text-gray-600 border-gray-200",
};

export default function AuditPage() {
  const params = useParams<{ id: string }>();
  const [run, setRun] = useState<AuditPageRun | null>(null);
  const [findings, setFindings] = useState<FindingSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [report, setReport] = useState<ScoreReport | null>(null);

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
      <main className="mx-auto max-w-2xl p-6">
        <p className="rounded bg-red-50 p-3 text-red-700">{error}</p>
      </main>
    );
  }
  if (!run) {
    return (
      <main className="mx-auto max-w-2xl p-6">
        <p className="text-gray-500">加载中…</p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-2xl p-6">
      <h1 className="mb-1 text-2xl font-semibold">审计 {STATUS_LABEL[run.status] ?? run.status}</h1>
      <p className="mb-6 text-xs text-gray-400">
        标准 {run.standard.code} {run.standard.version} ·{" "}
        {new Date(run.created_at).toLocaleString()}
      </p>

      {run.status === "FAILED" && (
        <p className="mb-4 rounded bg-red-50 p-3 text-red-700">
          审计失败：{run.status_reason ?? "未知原因"}（原始证据已保留，可重新启动审计）
        </p>
      )}
      {!TERMINAL.has(run.status) && (
        <p className="mb-4 rounded bg-blue-50 p-3 text-blue-700">AI 分析进行中，页面自动刷新…</p>
      )}
      {run.status === "REVIEW_REQUIRED" && (
        <section className="mb-6 rounded border p-4">
          <h2 className="mb-2 font-medium">报告</h2>
          <button
            className="rounded bg-black px-4 py-2 font-medium text-white"
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
            <div className="mt-3 text-sm">
              <p className="font-semibold">
                总分：
                {report.total_score === null
                  ? "部分评估（未出总分）"
                  : `${report.total_score} / 100`}
                <span className="ml-2 text-gray-500">覆盖率 {report.coverage_pct}%</span>
              </p>
              <ul className="mt-2 space-y-1">
                {report.dimensions.map((d) => (
                  <li key={d.dimension}>
                    {d.dimension}: {d.score}
                  </li>
                ))}
              </ul>
              {report.blocking.length > 0 && (
                <p className="mt-2 text-red-700">
                  阻断/关键问题：{report.blocking.map((b) => b.rule_id).join("、")}
                </p>
              )}
            </div>
          )}
        </section>
      )}

      {TERMINAL.has(run.status) && findings.length === 0 && (
        <p className="text-gray-500">本次审计没有产出候选结论。</p>
      )}

      <ul className="space-y-3">
        {findings.map((f) => (
          <li key={f.id}>
            <Link
              href={`/findings/${f.id}`}
              className={`block rounded border p-4 hover:opacity-90 ${FINDING_STYLE[f.assessment_status] ?? ""}`}
            >
              <div className="flex items-center justify-between">
                <span className="font-semibold">
                  {f.assessment_status} · {f.rule_id}
                </span>
                <span className="text-xs">
                  {f.review_status === "UNREVIEWED" ? "待人审" : `已${f.review_status}`}
                </span>
              </div>
              {f.observation && <p className="mt-1 text-sm">观察：{f.observation}</p>}
              <p className="mt-1 text-xs opacity-70">置信度 {f.confidence}</p>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}

type AuditPageRun = {
  id: string;
  status: string;
  status_reason: string | null;
  standard: { code: string; version: string };
  created_at: string;
};
