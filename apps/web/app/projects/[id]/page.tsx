"use client";

/**
 * Project detail: image upload (presigned PUT → server-side validation) and
 * audit launch — the write half of the P0-1/P0-2/Slice-1 flow.
 */

import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useRef, useState } from "react";

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
    <main className="mx-auto max-w-2xl p-6">
      <Link href="/dashboard" className="text-sm text-gray-500 hover:underline">
        ← 返回项目列表
      </Link>
      <h1 className="my-2 text-2xl font-semibold">{project?.name ?? "…"}</h1>

      {error && <p className="mb-4 rounded bg-red-50 p-3 text-red-700">{error}</p>}

      <section className="mb-8 rounded-lg border p-4">
        <h2 className="mb-3 font-medium">上传材料图片</h2>
        <input
          ref={fileInput}
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
          className="block w-full text-sm"
        />
        {uploading && <p className="mt-2 text-sm text-gray-500">上传并校验中…</p>}
        <ul className="mt-3 space-y-2">
          {evidence.map((e) => (
            <li key={e.id} className="rounded border px-3 py-2 text-sm">
              <span
                className={
                  e.processing_status === "READY"
                    ? "font-medium text-green-700"
                    : "font-medium text-red-700"
                }
              >
                {STATUS_LABEL[e.processing_status] ?? e.processing_status}
              </span>
              <span className="ml-2 text-gray-400">{e.sha256.slice(0, 12)}…</span>
            </li>
          ))}
          {evidence.length === 0 && <li className="text-sm text-gray-500">还没有上传任何材料。</li>}
        </ul>
      </section>

      <section className="mb-8 rounded-lg border p-4">
        <h2 className="mb-3 font-medium">AI 审计</h2>
        <button
          className="rounded bg-black px-4 py-2 font-medium text-white disabled:opacity-50"
          onClick={startAudit}
          disabled={starting || readyIds.length === 0}
        >
          {starting ? "启动中…" : `启动审计（${readyIds.length} 份就绪材料）`}
        </button>
        <ul className="mt-3 space-y-2">
          {audits.map((a) => (
            <li key={a.id} className="text-sm">
              <Link className="text-blue-700 hover:underline" href={`/audits/${a.id}`}>
                审计 {a.id.slice(0, 8)}…
              </Link>
              <span className="ml-2">{a.status}</span>
              <span className="ml-2 text-gray-400">{new Date(a.created_at).toLocaleString()}</span>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
