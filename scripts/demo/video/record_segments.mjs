/**
 * 录制 demo 视频的浏览器片段（Playwright recordVideo → webm）。
 * 前提：Web :3001（next dev，含工作区前端）、API :8081（演示修复后的代码）、
 *       AI :8100（真实模型）——与 functional_test 相同的隔离栈。
 *
 * 产物写入 scripts/demo/video/raw/*.webm。
 * 每个片段用 token 注入直接落到目标页（不含登录流程，画面与旁白一一对齐）；
 * 节奏刻意放慢（鼠标滑动、平滑滚动、停顿），供剪辑后配旁白。
 */
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
fs.mkdirSync(RAW, { recursive: true });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function authedCtx(browser, token) {
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
  page.setDefaultTimeout(20000);
  return { ctx, page };
}

async function driftTo(page, locator) {
  const box = await locator.boundingBox();
  if (!box) throw new Error("locator 不可见，无法漂移");
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 22 });
}

async function glideScroll(page, totalPx, step = 90, interval = 28) {
  for (let y = 0; y < totalPx; y += step) {
    await page.mouse.wheel(0, step);
    await sleep(interval);
  }
}

async function drift(page, x, y) {
  await page.mouse.move(x, y, { steps: 25 });
}

async function save(ctx, page, name) {
  const video = page.video();
  await ctx.close();
  fs.renameSync(await video.path(), path.join(RAW, name));
  console.log(`  🎬 ${name}`);
}

async function main() {
  const browser = await chromium.launch({ headless: true });

  const token = await (async () => {
    const r = await fetch(`${API}/internal/dev-token`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: "demo@local.dev" }),
    });
    return (await r.json()).token;
  })();
  const auth = { Authorization: `Bearer ${token}` };
  const projects = (await (await fetch(`${API}/projects`, { headers: auth })).json()).data;
  const shop = projects.find((p) => p.name.includes("全聚德"));
  const runs = (await (await fetch(`${API}/projects/${shop.id}/audits`, { headers: auth })).json())
    .data;
  const parentRun = runs.find((r) => !r.parent_run_id);
  const parentAudit = (
    await (await fetch(`${API}/audits/${parentRun.id}`, { headers: auth })).json()
  ).data;
  const d2 = parentAudit.findings.find((f) => f.rule_id === "IRRS-D2-001");

  // ---------- seg_dashboard：dashboard（项目列表） ----------
  {
    const { ctx, page } = await authedCtx(browser, token);
    await page.goto(`${WEB}/dashboard`, { waitUntil: "networkidle" });
    await sleep(2000);
    await drift(page, 640, 430);
    await sleep(1400);
    await drift(page, 640, 580);
    await sleep(1600);
    await save(ctx, page, "seg_dashboard.webm");
  }

  // ---------- seg_project：项目详情 + 证据 ----------
  {
    const { ctx, page } = await authedCtx(browser, token);
    await page.goto(`${WEB}/projects/${shop.id}`, { waitUntil: "networkidle" });
    await sleep(1800);
    await glideScroll(page, 340);
    await sleep(1300);
    await drift(page, 900, 520);
    await sleep(1000);
    await glideScroll(page, 260);
    await sleep(2000);
    await save(ctx, page, "seg_project.webm");
  }

  // ---------- seg_report：COMPLETED 审计的冻结报告 ----------
  {
    const { ctx, page } = await authedCtx(browser, token);
    await page.goto(`${WEB}/audits/${parentRun.id}`, { waitUntil: "networkidle" });
    await sleep(2400);
    await glideScroll(page, 320);
    await sleep(1600);
    await glideScroll(page, 360);
    await sleep(1800);
    await glideScroll(page, 340);
    await sleep(1800);
    await save(ctx, page, "seg_report.webm");
  }

  // ---------- seg_finding：证据查看器（D2-001，带 bbox） ----------
  {
    const { ctx, page } = await authedCtx(browser, token);
    await page.goto(`${WEB}/findings/${d2.id}`, { waitUntil: "domcontentloaded" });
    await page.waitForLoadState("networkidle");
    await page.waitForTimeout(2800); // 等证据签名 URL 图片加载
    await drift(page, 960, 520);
    await sleep(1800);
    await glideScroll(page, 320);
    await sleep(2200);
    await save(ctx, page, "seg_finding.webm");
  }

  // ---------- seg_review：国博新 findings 人审 + 任务推进 ----------
  {
    const { ctx, page } = await authedCtx(browser, token);
    const guobo = projects.find((p) => p.name.includes("博物馆"));
    const gruns = (await (await fetch(`${API}/projects/${guobo.id}/audits`, { headers: auth })).json()).data;
    const grate = gruns.find((r) => r.status === "REVIEW_REQUIRED");
    if (!grate) throw new Error("国博没有 REVIEW_REQUIRED 的 run，先等预跑完成");
    const gaudit = (await (await fetch(`${API}/audits/${grate.id}`, { headers: auth })).json()).data;
    const fresh = (gaudit.findings ?? []).find(
      (f) => f.review_status === "UNREVIEWED" && (f.evidence_refs ?? []).length > 0,
    ) ?? (gaudit.findings ?? []).find((f) => f.review_status === "UNREVIEWED");
    if (!fresh) throw new Error("国博该 run 没有 UNREVIEWED 的 finding 了");
    await page.goto(`${WEB}/findings/${fresh.id}`, { waitUntil: "networkidle" });
    await sleep(2000);
    // 确认（鼠标先漂到按钮再点，节奏自然）
    const confirmBtn = page.getByRole("button", { name: "确认", exact: true });
    await confirmBtn.scrollIntoViewIfNeeded();
    await driftTo(page, confirmBtn);
    await sleep(600);
    await confirmBtn.click();
    await page.getByText(/已处置/).waitFor({ timeout: 8000 });
    await sleep(1600);
    // 任务推进：确认整改 → 开始整改 → 已改好，申请复测
    for (const label of ["确认整改", "开始整改", "已改好，申请复测"]) {
      const btn = page.getByRole("button", { name: label });
      await btn.scrollIntoViewIfNeeded();
      await driftTo(page, btn);
      await sleep(450);
      await btn.click();
      await sleep(1300);
    }
    await save(ctx, page, "seg_review.webm");
  }

  // ---------- seg_diff：复测 run 的 Before/After（见 rerecord_diff.mjs） ----------

  await browser.close();
  console.log("segments done ✅");
}

main().catch((e) => {
  console.error("❌", e.message);
  process.exit(1);
});
