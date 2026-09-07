import type { Metadata } from "next";
import type { ReactNode } from "react";

import "@/styles/tokens.css";
import "@/styles/globals.css";
import "@/styles/components.css";

export const metadata: Metadata = {
  title: "Arrival Ready ｜迎客验收",
  description:
    "在国际访客真正到来之前，验证服务体验是否可用，并把问题变成可追踪、可整改、可复测的任务。",
};

// 首帧前同步主题（localStorage 优先，其次跟随系统），避免深色用户看到白屏闪烁。
const themeInit = `(()=>{try{const s=localStorage.getItem("fg-theme");document.documentElement.dataset.theme=s||(matchMedia("(prefers-color-scheme: dark)").matches?"dark":"light")}catch{}})()`;

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN" data-theme="light" suppressHydrationWarning>
      <body>
        <script dangerouslySetInnerHTML={{ __html: themeInit }} />
        {children}
      </body>
    </html>
  );
}
