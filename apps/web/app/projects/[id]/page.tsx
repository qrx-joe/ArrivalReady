"use client";

/**
 * Project detail: image upload (presigned PUT → server-side validation) and
 * audit launch — the write half of the P0-1/P0-2/Slice-1 flow.
 */

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

import { AppShell } from "@/components/layout/AppShell";
import { ApiError, api, getToken } from "@/lib/api";

type Evidence = {
  id: string;
  type: string;
  processing_status: string;
  sha256: string;
  created_at: string;
};
type AuditRun = { id: string; status: string; status_reason: string | null; created_at: string };

async function sha256Hex(file: File): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return Array.from(new Uint8Array(digest))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

const STATUS_LABEL: Record<string, string> = {
  PENDING_UPLOAD: "待上传",
  VALIDATING: "校验中",
  READY: "就绪",
  QUARANTINED: "已隔离",
  DELETED: "已删除",
};

const STATUS_BADGE: Record<string, string> = {
  READY: "success",
  QUARANTINED: "danger",
  VALIDATING: "warning",
};

export default function ProjectPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const fileInput = useRef<HTMLInputElement>(null);

  const [project, setProject] = useState<{ name: string } | null>(null);
  const [evidence, setEvidence] = useState<Evidence[]>([]);
  const [audits, setAudits] = useState<AuditRun[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const [starting, setStarting] = useState(false);

  const reload = useCallback(async () => {
    setEvidence(await api.listEvidence(params.id));
    setAudits(await api.listAudits(params.id));
  }, [params.id]);

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    api
      .getProject(params.id)
      .then(setProject)
      .catch((e: ApiError) => setError(e.message));
    reload().catch((e: ApiError) => setError(e.message));
  }, [params.id, reload, router]);

  const upload = useCallback(
    async (file: File) => {
      setUploading(true);
      setError(null);
      try {
        const sha = await sha256Hex(file);
        const presigned = await api.createUploadURL(params.id, {
          evidence_type: "image",
          mime_type: file.type || "image/jpeg",
          size_bytes: file.size,
          journey_stage: "understand",
        });
        const put = await fetch(presigned.upload_url, {
          method: "PUT",
          headers: { "Content-Type": file.type || "image/jpeg" },
          body: file,
        });
        if (!put.ok) throw new ApiError(put.status, "上传到对象存储失败");
        const done = await api.completeUpload(params.id, {
          evidence_id: presigned.evidence_id,
          object_key: presigned.object_key,
          sha256: sha,
          idempotencyKey: crypto.randomUUID(),
        });
        if (done.processing_status === "QUARANTINED") {
          setError("文件未通过服务端校验，已被隔离（类型或内容不符）");
        }
        await reload();
      } catch (e) {
        setError(e instanceof ApiError ? e.message : "上传失败");
      } finally {
        setUploading(false);
        if (fileInput.current) fileInput.current.value = "";
      }
    },
    [params.id, reload],
  );

  const readyIds = evidence.filter((e) => e.processing_status === "READY").map((e) => e.id);

  const startAudit = useCallback(async () => {
    setStarting(true);
    setError(null);
    try {
      const run = await api.createAudit(params.id, readyIds);
      router.push(`/audits/${run.id}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "启动审计失败");
      setStarting(false);
    }
  }, [params.id, readyIds, router]);

  return (
    <AppShell
      crumb={
        <>
          项目 / <b>{project?.name ?? "…"}</b>
        </>
      }
    >
      <div className="app-heading">
        <div>
          <h1>{project?.name ?? "…"}</h1>
          <p>证据库与 AI 验收运行</p>
        </div>
      </div>

      {error && (
        <div className="alert danger" role="alert" style={{ marginBottom: "var(--s4)" }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      )}

      <div className="grid-2" style={{ alignItems: "start" }}>
        <section className="panel">
          <div className="panel-head">
            <h2>上传材料图片</h2>
            <span>门头 / 菜单 / 标识等现场材料</span>
          </div>
          <div className="panel-body">
            <input
              ref={fileInput}
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
              className="file-input"
            />
            {uploading && (
              <p className="muted" style={{ marginTop: "var(--s2)" }}>
                上传并校验中…
              </p>
            )}
            <ul className="stack" style={{ marginTop: "var(--s3)" }}>
              {evidence.map((e) => (
                <li
                  key={e.id}
                  className="row"
                  style={{
                    justifyContent: "space-between",
                    border: "1px solid var(--border)",
                    borderRadius: "var(--r-md)",
                    padding: "9px 12px",
                    fontSize: "var(--text-sm)",
                  }}
                >
                  <span className="row">
                    <span className={`badge ${STATUS_BADGE[e.processing_status] ?? "neutral"}`}>
                      {STATUS_LABEL[e.processing_status] ?? e.processing_status}
                    </span>
                    <span className="mono muted">{e.sha256.slice(0, 12)}…</span>
                  </span>
                </li>
              ))}
              {evidence.length === 0 && (
                <li className="muted">还没有上传任何材料。第一张门头或菜单照片即可开始。</li>
              )}
            </ul>
          </div>
        </section>

        <section className="panel">
          <div className="panel-head">
            <h2>AI 验收</h2>
            <span>同标准复测才可比较</span>
          </div>
          <div className="panel-body">
            <button
              className="btn btn-primary"
              onClick={startAudit}
              disabled={starting || readyIds.length === 0}
            >
              {starting ? "启动中…" : `启动审计（${readyIds.length} 份就绪材料）`}
            </button>
            <ul className="stack" style={{ marginTop: "var(--s3)" }}>
              {audits.map((a) => (
                <li key={a.id} className="row" style={{ fontSize: "var(--text-sm)" }}>
                  <Link
                    className="back-link"
                    style={{ color: "var(--brand-strong)" }}
                    href={`/audits/${a.id}`}
                  >
                    审计 {a.id.slice(0, 8)}…
                  </Link>
                  <span className="badge neutral">{a.status}</span>
                  <span className="muted">{new Date(a.created_at).toLocaleString()}</span>
                </li>
              ))}
              {audits.length === 0 && (
                <li className="muted">还没有验收记录——就绪材料后点上面的按钮启动。</li>
              )}
            </ul>
          </div>
        </section>
      </div>
    </AppShell>
  );
}
