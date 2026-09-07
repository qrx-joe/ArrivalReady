"use client";

/**
 * Dev login: mints the local test identity via the Go API's hard-gated
 * /internal/dev-token (only exists with ARRIVAL_ENABLE_TEST_IDENTITY=1).
 * Real OIDC replaces this page in B05's production path.
 */

import { useRouter } from "next/navigation";
import { useState } from "react";

import { api, getToken, setToken } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (getToken()) {
    router.replace("/dashboard");
  }

  const login = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await api.loginDev("dev@arrivalready.local");
      setToken(res.token);
      router.replace("/dashboard");
    } catch {
      setError(
        "登录失败：请确认 Go API 已以本地测试身份模式启动（ARRIVAL_ENABLE_TEST_IDENTITY=1）",
      );
      setBusy(false);
    }
  };

  return (
    <main className="login-wrap">
      <section className="login-card">
        <div className="brandlockup">
          <span className="brandmark" aria-hidden="true">
            <svg
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.8"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M6 3.7h12a2 2 0 0 1 2 2v12.6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5.7a2 2 0 0 1 2-2Z" />
              <path d="M8 8h8M8 12h5M8 16h3" />
              <path d="m15.5 15.5 1.3 1.3 2.7-3" />
            </svg>
          </span>
          <span className="brandname">Arrival Ready</span>
        </div>
        <div>
          <h1 style={{ fontSize: "var(--text-xl)", letterSpacing: "-.03em" }}>迎客验收</h1>
          <p className="muted" style={{ marginTop: 4 }}>
            国际访客接待准备度验收 · 本地开发登录
          </p>
        </div>
        <button className="btn btn-primary" onClick={login} disabled={busy}>
          {busy ? "登录中…" : "开发者登录（本地测试身份）"}
        </button>
        {error && (
          <div className="alert danger" role="alert">
            <span>!</span>
            <span>{error}</span>
          </div>
        )}
      </section>
    </main>
  );
}
