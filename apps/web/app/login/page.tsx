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
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center p-6">
      <h1 className="mb-1 text-2xl font-semibold">Arrival Ready</h1>
      <p className="mb-6 text-sm text-gray-500">国际访客接待准备度验收 · 本地开发登录</p>
      <button
        className="rounded bg-black px-4 py-3 font-medium text-white disabled:opacity-50"
        onClick={login}
        disabled={busy}
      >
        {busy ? "登录中…" : "开发者登录（本地测试身份）"}
      </button>
      {error && <p className="mt-4 rounded bg-red-50 p-3 text-red-700">{error}</p>}
    </main>
  );
}
