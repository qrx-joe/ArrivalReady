"use client";

/**
 * AppShell：侧边栏 + 顶栏 + 内容区（设计规范 §03 布局与信息架构）。
 * 导航只放真实存在的路由；未实现的能力显式标注「规划中」，
 * 不做成可点击的死链接（不宣称完成）。
 */

import Link from "next/link";
import { usePathname } from "next/navigation";
import { type ReactNode, useCallback, useEffect, useState } from "react";

const BRAND_SVG = (
  <svg
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="1.8"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    <path d="M6 3.7h12a2 2 0 0 1 2 2v12.6a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V5.7a2 2 0 0 1 2-2Z" />
    <path d="M8 8h8M8 12h5M8 16h3" />
    <path d="m15.5 15.5 1.3 1.3 2.7-3" />
  </svg>
);

function NavIcon({ d }: { d: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={d} />
    </svg>
  );
}

function ThemeToggle() {
  const [theme, setTheme] = useState<"light" | "dark">("light");

  useEffect(() => {
    setTheme(document.documentElement.dataset.theme === "dark" ? "dark" : "light");
  }, []);

  const toggle = useCallback(() => {
    const next = theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try {
      localStorage.setItem("fg-theme", next);
    } catch {
      // 存储不可用时主题仍即时生效，仅不持久化。
    }
    setTheme(next);
  }, [theme]);

  return (
    <button className="icon-btn" type="button" onClick={toggle} aria-label="切换深浅色模式">
      {theme === "dark" ? (
        <NavIcon d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10Zm0-5v2m0 16v2M4.2 4.2l1.4 1.4m12.8 12.8 1.4 1.4M2 12h2m16 0h2M4.2 19.8l1.4-1.4M18.4 5.6l1.4-1.4" />
      ) : (
        <NavIcon d="M21 12.8A9 9 0 1 1 11.2 3 7 7 0 0 0 21 12.8Z" />
      )}
    </button>
  );
}

export type AppShellProps = {
  crumb?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
};

export function AppShell({ crumb, actions, children }: AppShellProps) {
  const pathname = usePathname();

  return (
    <div className="app">
      <aside className="app-sidebar">
        <Link className="app-brand" href="/dashboard" style={{ textDecoration: "none" }}>
          <span className="brandmark">{BRAND_SVG}</span>
          <span className="brandname">Arrival Ready</span>
        </Link>

        <div className="nav-group">
          <div className="nav-group-label">Workspace</div>
          <nav className="app-nav">
            <Link className={pathname === "/dashboard" ? "active" : ""} href="/dashboard">
              <NavIcon d="M4 6h16M4 12h16M4 18h10" />
              验收项目
            </Link>
            <button type="button" disabled title="验收运行历史（规划中）">
              <NavIcon d="m8 12 2.5 2.5L16 9" />
              验收记录
              <span className="nav-soon">规划中</span>
            </button>
            <button type="button" disabled title="整改任务看板（规划中）">
              <NavIcon d="M4 19V8m5 11V5m5 14v-7m5 7V3" />
              整改任务
              <span className="nav-soon">规划中</span>
            </button>
            <button type="button" disabled title="跨项目趋势（规划中）">
              <NavIcon d="M4 4h16v16H4z" />
              报告与趋势
              <span className="nav-soon">规划中</span>
            </button>
          </nav>
        </div>

        <div className="sidebar-bottom">
          <nav className="app-nav">
            <button type="button" disabled title="工作区设置（规划中）">
              <NavIcon d="M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8Zm8.4 4a8.4 8.4 0 0 0-.1-1.3l2-1.5-2-3.4-2.3 1a8.3 8.3 0 0 0-2.2-1.3L15.4 3h-4l-.4 2.5a8.3 8.3 0 0 0-2.2 1.3l-2.3-1-2 3.4 2 1.5a8.4 8.4 0 0 0 0 2.6l-2 1.5 2 3.4 2.3-1a8.3 8.3 0 0 0 2.2 1.3l.4 2.5h4l.4-2.5a8.3 8.3 0 0 0 2.2-1.3l2.3 1 2-3.4-2-1.5c.06-.43.1-.86.1-1.3Z" />
              设置
              <span className="nav-soon">规划中</span>
            </button>
            <ThemeToggle />
          </nav>
        </div>
      </aside>

      <div className="app-main">
        <header className="app-top">
          <div className="crumb">{crumb ?? "Arrival Ready"}</div>
          <div className="app-actions">{actions}</div>
        </header>
        <main className="app-content">{children}</main>
      </div>
    </div>
  );
}
