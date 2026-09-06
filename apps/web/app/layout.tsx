import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  title: "Arrival Ready ｜迎客验收",
  description:
    "在国际访客真正到来之前，验证服务体验是否可用，并把问题变成可追踪、可整改、可复测的任务。",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>{children}</body>
    </html>
  );
}
