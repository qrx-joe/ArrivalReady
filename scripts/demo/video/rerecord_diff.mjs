/** 补录 seg_diff：全聚德 COMPLETED 复测 run 的 Before/After 对比。 */
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
  const shop = projects.find((p) => p.name.includes("全聚德"));
  const runs = (await (await fetch(`${API}/projects/${shop.id}/audits`, { headers: auth })).json()).data;
  const child = runs.find((r) => r.parent_run_id && r.status === "COMPLETED");
  console.log("child run:", child.id);

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
  await page.goto(`${WEB}/audits/${child.id}`, { waitUntil: "networkidle" });
  await sleep(2400);
  const btn = page.getByRole("button", { name: /与上一轮对比/ });
  await btn.scrollIntoViewIfNeeded();
  const box = await btn.boundingBox();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 22 });
  await sleep(600);
  await btn.click();
  await sleep(1500);
  for (let y = 0; y < 330; y += 90) {
    await page.mouse.wheel(0, 90);
    await sleep(30);
  }
  await sleep(2200);
  const video = page.video();
  await ctx.close();
  fs.renameSync(await video.path(), path.join(RAW, "seg_diff.webm"));
  await browser.close();
  console.log("seg_diff.webm ✅");
}
main().catch((e) => {
  console.error("❌", e.message);
  process.exit(1);
});
