/** 补录 seg_analyzing：项目页点击「启动审计」→ 审计页「AI 分析中」轮询。 */
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const webRequire = createRequire(new URL("../../../apps/web/package.json", import.meta.url));
const { chromium } = webRequire("@playwright/test");

const WEB = process.env.FT_WEB ?? "http://localhost:3001";
const API = process.env.FT_API ?? "http://localhost:8081/api/v1";
const HERE = path.dirname(fileURLToPath(import.meta.url));
const RAW = path.join(HERE, "raw");
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function main() {
  const token = (
    await (
      await fetch(`${API}/internal/dev-token`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "demo@local.dev" }),
      })
    ).json()
  ).token;
  const auth = { Authorization: `Bearer ${token}` };
  const projects = (await (await fetch(`${API}/projects`, { headers: auth })).json()).data;
  const guobo = projects.find((p) => p.name.includes("博物馆"));

  const browser = await chromium.launch({ headless: true });
  const ctx = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    deviceScaleFactor: 2,
    locale: "zh-CN",
    recordVideo: { dir: RAW, size: { width: 1920, height: 1080 } },
  });
  await ctx.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    route.continue({ url: API + url.pathname.replace(/^.*\/api\/v1/, "") + url.search });
  });
  await ctx.addInitScript((t) => localStorage.setItem("arrivalready.token", t), token);
  const page = await ctx.newPage();

  await page.goto(`${WEB}/projects/${guobo.id}`, { waitUntil: "networkidle" });
  await page.waitForLoadState("networkidle");
  await sleep(1800);
  const startBtn = page.getByRole("button", { name: /启动审计/ });
  await startBtn.scrollIntoViewIfNeeded();
  const box = await startBtn.boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 24 });
  await sleep(700);
  await startBtn.click();
  await page.waitForURL("**/audits/**", { timeout: 20000 });
  await page.waitForLoadState("networkidle");
  await sleep(2500);
  await page.getByText(/AI 分析进行中/).waitFor({ timeout: 10000 });
  await sleep(6000); // 录住「分析中，页面自动刷新」的状态

  const video = page.video();
  await ctx.close();
  fs.renameSync(await video.path(), path.join(RAW, "seg_analyzing.webm"));
  await browser.close();
  console.log("seg_analyzing.webm ✅");
}
main().catch((e) => {
  console.error("❌", e.message);
  process.exit(1);
});
