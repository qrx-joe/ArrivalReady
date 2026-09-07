"use client";

/**
 * Evidence Viewer — the Magic Moment (PRD §12.4): every finding is human
 * verifiable in one click. The bounding box is drawn in ORIGINAL-image
 * normalized coordinates, so it stays anchored at any zoom or screen size
 * (定位随缩放保持). Rule, observation vs inference, confidence and the
 * 待人审 badge are all explicit (B09 步骤④).
 */

import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

import { ApiError, api, getToken } from "@/lib/api";

type Finding = {
  id: string;
  rule_id: string;
  assessment_status: string;
  severity: string;
  observation: string | null;
  reason: string | null;
  recommended_fix: string | null;
  confidence: number;
  review_status: string;
  evidence_refs: {
    evidence_id: string;
    locator: { type: string; bbox?: { x: number; y: number; w: number; h: number } };
  }[];
};

type EvidenceImage = { id: string; url: string | null; status: string };

export default function FindingPage() {
  const params = useParams<{ id: string }>();
  const [finding, setFinding] = useState<Finding | null>(null);
  const [images, setImages] = useState<EvidenceImage[]>([]);
  const [error, setError] = useState<string | null>(null);

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
      <main className="mx-auto max-w-2xl p-6">
        <p className="rounded bg-red-50 p-3 text-red-700">{error}</p>
      </main>
    );
  }
  if (!finding) {
    return (
      <main className="mx-auto max-w-2xl p-6">
        <p className="text-gray-500">加载中…</p>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-2xl p-6">
      <Link href="javascript:history.back()" className="text-sm text-gray-500 hover:underline">
        ← 返回
      </Link>
      <h1 className="my-2 text-2xl font-semibold">
        {finding.assessment_status} · {finding.rule_id}
      </h1>
      <div className="mb-4 flex gap-2 text-sm">
        <span className="rounded border px-2 py-0.5">严重度 {finding.severity}</span>
        <span className="rounded border px-2 py-0.5">置信度 {finding.confidence}</span>
        <span
          className={`rounded border px-2 py-0.5 ${
            finding.review_status === "UNREVIEWED" ? "border-blue-300 bg-blue-50 text-blue-700" : ""
          }`}
        >
          {finding.review_status === "UNREVIEWED"
            ? "待人审（AI 候选，非最终结论）"
            : `人审：${finding.review_status}`}
        </span>
      </div>

      {images.map((img, i) => {
        const bbox = finding.evidence_refs[i]?.locator.bbox;
        return (
          <figure key={img.id} className="relative mb-4">
            {img.url ? (
              <span className="relative block">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={img.url} alt={`证据 ${i + 1}`} className="w-full rounded border" />
                {bbox && (
                  <span
                    className="absolute border-2 border-red-500 bg-red-500/10"
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
              <p className="rounded bg-gray-50 p-3 text-sm text-gray-600">
                证据不可查看（状态：{img.status}）。hash 与元数据已保留。
              </p>
            )}
          </figure>
        );
      })}

      <section className="space-y-2 text-sm">
        <p>
          <span className="font-medium">观察（事实）：</span>
          {finding.observation ?? "—"}
        </p>
        <p>
          <span className="font-medium">推断（理由）：</span>
          {finding.reason ?? "—"}
        </p>
        {finding.recommended_fix && (
          <p>
            <span className="font-medium">建议整改：</span>
            {finding.recommended_fix}
          </p>
        )}
        <p className="rounded bg-yellow-50 p-3 text-yellow-800">
          以上是 AI 候选结论；确认、修改或驳回后才计入报告（AI Suggests, Human Owns）。
        </p>
      </section>
    </main>
  );
}
