"use client";

/**
 * Dashboard: project list + creation. The entry surface of the P0-1 flow.
 */

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { ApiError, api, getToken } from "@/lib/api";

type Project = { id: string; name: string; entity_type: string; created_at: string };

export default function DashboardPage() {
  const router = useRouter();
  const [projects, setProjects] = useState<Project[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!getToken()) {
      router.replace("/login");
      return;
    }
    // Fetch once on mount; `router` is intentionally NOT a dependency — its
    // identity changes per render in the App Router, which would loop this
    // fetch forever and keep remounting the form (E2E 实测).
    api
      .listProjects()
      .then((list) => setProjects(list ?? []))
      .catch((e: ApiError) => setError(e.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const create = useCallback(async () => {
    if (!name.trim()) return;
    setCreating(true);
    setError(null);
    try {
      const p = await api.createProject({
        name: name.trim(),
        entity_type: "restaurant",
        target_locale: "en-US",
      });
      router.push(`/projects/${p.id}`);
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "创建失败");
      setCreating(false);
    }
  }, [name, router]);

  return (
    <main className="mx-auto max-w-2xl p-6">
      <h1 className="mb-1 text-2xl font-semibold">验收项目</h1>
      <p className="mb-6 text-sm text-gray-500">创建项目 → 上传材料 → AI 审计 → 查看带证据的结论</p>

      <section className="mb-8 rounded-lg border p-4">
        <h2 className="mb-3 font-medium">新建项目</h2>
        <div className="flex flex-col gap-2 sm:flex-row">
          <input
            className="flex-1 rounded border px-3 py-2"
            placeholder="门店名称（如：某某小馆）"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && create()}
          />
          <button
            className="rounded bg-black px-4 py-2 font-medium text-white disabled:opacity-50"
            onClick={create}
            disabled={creating || !name.trim()}
          >
            {creating ? "创建中…" : "创建"}
          </button>
        </div>
      </section>

      {error && <p className="mb-4 rounded bg-red-50 p-3 text-red-700">{error}</p>}

      {projects === null && !error && <p className="text-gray-500">加载中…</p>}
      {projects !== null && projects.length === 0 && (
        <p className="text-gray-500">还没有项目——从上面的表单创建第一个。</p>
      )}
      <ul className="space-y-2">
        {projects?.map((p) => (
          <li key={p.id}>
            <Link className="block rounded border p-3 hover:bg-gray-50" href={`/projects/${p.id}`}>
              <span className="font-medium">{p.name}</span>
              <span className="ml-2 text-xs text-gray-400">{p.entity_type}</span>
            </Link>
          </li>
        ))}
      </ul>
    </main>
  );
}
