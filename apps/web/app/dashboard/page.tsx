"use client";

/**
 * Dashboard: project list + creation. The entry surface of the P0-1 flow.
 */

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

import { AppShell } from "@/components/layout/AppShell";
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
    <AppShell
      crumb={
        <>
          项目 / <b>全部</b>
        </>
      }
    >
      <div className="app-heading">
        <div>
          <h1>验收项目</h1>
          <p>创建项目 → 上传材料 → AI 审计 → 查看带证据的结论</p>
        </div>
      </div>

      <section className="card" style={{ marginBottom: "var(--s6)" }}>
        <h2 className="card-title">新建项目</h2>
        <p className="card-copy" style={{ marginBottom: "var(--s3)" }}>
          先建一个门店/场馆档案，后续所有证据、审计与整改都挂在它下面。
        </p>
        <div className="form-row">
          <input
            className="input"
            placeholder="门店名称（如：某某小馆）"
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => e.key === "Enter" && create()}
          />
          <button className="btn btn-primary" onClick={create} disabled={creating || !name.trim()}>
            {creating ? "创建中…" : "创建"}
          </button>
        </div>
      </section>

      {error && (
        <div className="alert danger" role="alert" style={{ marginBottom: "var(--s4)" }}>
          <span>!</span>
          <span>{error}</span>
        </div>
      )}

      {projects === null && !error && (
        <div aria-busy="true">
          <div className="skeleton" style={{ width: "72%" }} />
          <div className="skeleton" />
          <div className="skeleton" style={{ width: "58%" }} />
        </div>
      )}
      {projects !== null && projects.length === 0 && (
        <p className="muted">还没有项目——从上面的表单创建第一个。</p>
      )}
      <ul className="project-list">
        {projects?.map((p) => (
          <li key={p.id}>
            <Link className="project-card" href={`/projects/${p.id}`}>
              <span className="name">{p.name}</span>
              <span className="badge neutral">{p.entity_type}</span>
            </Link>
          </li>
        ))}
      </ul>
    </AppShell>
  );
}
