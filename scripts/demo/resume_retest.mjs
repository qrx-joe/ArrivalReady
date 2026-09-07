/**
 * 续跑：复测 run 已到 REVIEW_REQUIRED 时的收尾验证。
 *   快速确认全部复测 findings → 加载 Before/After Diff →
 *   对第一轮 D2-001 finding 的任务推进到 READY_FOR_RETEST →
 *   复测 PASS 已确认 → RESOLVED 应当放行（闭环）。
 * 与 functional_test.mjs 同一套路由拦截（浏览器 API 指向 :8081）。
 */
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const webRequire = createRequire(new URL("../../apps/web/package.json", import.meta.url));
const { chromium } = webRequire("@playwright/test");

const WEB = process.env.FT_WEB ?? "http://localhost:3001";
const API = process.env.FT_API ?? "http://localhost:8081/api/v1";
const DEMO_DIR = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.resolve(DEMO_DIR, "artifacts");

async function shot(page, name) {
  const file = path.join(OUT, `${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  console.log(`  📸 ${path.basename(file)}`);
}

async function api(token, method, p, body) {
  const r = await fetch(`${API}${p}`, {
    method,
    headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await r.text();
  return { status: r.status, json: text ? JSON.parse(text) : null };
}

async function main() {
  const browser = await chromium.launch({ headless: false, slowMo: 100 });
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: "zh-CN" });
  await context.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    route.continue({ url: API + url.pathname.replace(/^.*\/api\/v1/, "") + url.search });
  });
  const page = await context.newPage();

  // 登录
  await page.goto(`${WEB}/login`, { waitUntil: "networkidle" });
  await page.getByRole("button", { name: /开发者登录/ }).click();
  await page.waitForURL("**/dashboard");
  const token = await page.evaluate(() => localStorage.getItem("arrivalready.token"));

  // 找到全聚德项目 → 最新 run（复测，REVIEW_REQUIRED）
  const projects = (await api(token, "GET", "/projects")).json.data;
  const shop = projects.find((p) => p.name.includes("全聚德"));
  const runs = (await api(token, "GET", `/projects/${shop.id}/audits`)).json.data;
  const child = runs[0]; // created_at DESC
  console.log(`复测 run ${child.id.slice(0, 8)} status=${child.status} parent=${String(child.parent_run_id).slice(0, 8)}`);
  if (!["REVIEW_REQUIRED", "COMPLETED"].includes(child.status))
    throw new Error("复测 run 状态不符: " + child.status);

  // 1) 打开复测审计页 → 快速确认所有 UNREVIEWED
  await page.goto(`${WEB}/audits/${child.id}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(1200);
  await shot(page, "retest_review_required");
  let n = 0;
  for (;;) {
    const btn = page.locator("button.finding-quick-confirm").first();
    if ((await btn.count()) === 0) break;
    await btn.click();
    await page.waitForTimeout(350);
    n++;
  }
  console.log(`  ✓ 快速确认 ${n} 条`);
  await page.waitForTimeout(600);

  // 2) finalize（冻结评分）→ COMPLETED → Before/After Diff
  const finalizeBtn = page.getByRole("button", { name: /生成报告（冻结评分）/ });
  if (await finalizeBtn.count()) {
    await finalizeBtn.click();
    await page.getByText(/报告（已冻结）/).waitFor({ timeout: 8000 });
    console.log("  ✓ 复测 finalize 完成，冻结报告可见");
  } else {
    console.log("  · run 已 COMPLETED，跳过 finalize");
  }
  await page.waitForTimeout(600);
  await shot(page, "retest_finalized_report");
  await page.getByRole("button", { name: /与上一轮对比/ }).click();
  await page.waitForTimeout(900);
  await shot(page, "retest_diff_table");
  const rows = await page.locator("table.table tbody tr").count();
  console.log(`  ✓ Diff ${rows} 行`);
  await page.evaluate(() => window.scrollTo(0, 0));
  await shot(page, "retest_diff_full");

  // 3) 第一轮 IRRS-D2-001 finding 任务推进 + RESOLVED 闭环（幂等：已是终态则跳过）
  const parentRunId = child.parent_run_id;
  const audit = (await api(token, "GET", `/audits/${parentRunId}`)).json.data;
  const d2 = audit.findings.find((f) => f.rule_id === "IRRS-D2-001");
  console.log(`第一轮 D2-001 finding=${d2.id.slice(0, 8)} review=${d2.review_status}`);
  let v = d2.task.version;
  let status = d2.task.workflow_status;
  if (status !== "RESOLVED") {
    const chain = { OPEN: ["ACKNOWLEDGED"], ACKNOWLEDGED: ["FIXING"], FIXING: ["READY_FOR_RETEST"], READY_FOR_RETEST: [] };
    for (const to of chain[status] ?? []) {
      const r = await api(token, "PATCH", `/findings/${d2.id}/task`, { workflow_status: to, version: v });
      if (r.status !== 200) throw new Error(`任务推进 ${status}→${to} 失败: ${r.status} ${JSON.stringify(r.json)}`);
      status = to;
      v = r.json.data.version;
    }
    console.log(`  ✓ 任务推进到 READY_FOR_RETEST (v${v})`);
  } else {
    console.log("  · 任务已 RESOLVED，跳过推进");
  }
  const childD2 = (await api(token, "GET", `/audits/${child.id}`)).json.data.findings.find(
    (f) => f.rule_id === "IRRS-D2-001",
  );
  console.log(`复测 D2-001: ${childD2.assessment_status} review=${childD2.review_status}`);
  if (childD2.assessment_status !== "PASS") throw new Error("复测 D2-001 非 PASS，无法验证 RESOLVED 闭环");
  if (childD2.review_status === "UNREVIEWED") {
    const r = await api(token, "POST", `/findings/${childD2.id}/reviews`, { decision: "confirm" });
    if (r.status !== 200) throw new Error("复测确认失败: " + r.status);
  }
  const rrStatus = d2.task.workflow_status;
  if (rrStatus === "RESOLVED") {
    console.log("  · RESOLVED 已验证过，跳过");
  } else {
    const rr = await api(token, "PATCH", `/findings/${d2.id}/task`, { workflow_status: "RESOLVED", version: v });
    if (rr.status !== 200) throw new Error(`RESOLVED 被拒: ${rr.status} ${JSON.stringify(rr.json)}`);
    console.log(`  ✓ RESOLVED 放行 (v${rr.json.data.version})——复测通过驱动闭环成立`);
  }

  // 打开第一轮 D2 finding 详情页截图（任务面板显示 RESOLVED）
  await page.goto(`${WEB}/findings/${d2.id}`, { waitUntil: "networkidle" });
  await page.waitForTimeout(800);
  await shot(page, "task_resolved");
  await browser.close();
  console.log("RESUME ALL GREEN ✅");
}

main().catch((e) => {
  console.error("\n❌ FAIL:", e.message);
  process.exit(1);
});
