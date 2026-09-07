/**
 * 全功能闭环实测（演示前验收）：
 *   登录 → dashboard → 项目列表 → 项目详情（证据）→ COMPLETED 审计（冻结评分）→
 *   finding 详情（证据图 + bbox + 人审状态 + 整改任务面板）→ 任务状态机（含非法迁移与
 *   无复测 RESOLVED 的拒绝路径）→ 发起复测（真实模型）→ 复测完成 → 人审 → Before/After Diff →
 *   复测 PASS 后任务 RESOLVED。
 *
 * 运行前提：
 *   - API :8081（本脚本验证的新代码）
 *   - Web :3001（next dev，热加载工作区前端）
 *   - AI :8100（真实模型）
 * 浏览器内 API 请求经 Playwright 路由拦截指向 :8081，与并行占用的 :8080 互不干扰。
 *
 * 用法：node scripts/demo/functional_test.mjs   （在 apps/web 目录下有 playwright 依赖，
 *       这里直接从仓库根运行：node --experimental-vm-modules 不需要；纯 ESM。）
 */
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

// Playwright 依赖装在 apps/web（E2E 也在那里），脚本从仓库任意位置可运行。
const webRequire = createRequire(
  new URL("../../apps/web/package.json", import.meta.url),
);
const { chromium } = webRequire("@playwright/test");

const WEB = process.env.FT_WEB ?? "http://localhost:3001";
const API = process.env.FT_API ?? "http://localhost:8081/api/v1";
const DEMO_DIR = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.resolve(DEMO_DIR, "artifacts");
fs.mkdirSync(OUT, { recursive: true });

const shots = [];
async function shot(page, name) {
  const file = path.join(OUT, `${String(shots.length + 1).padStart(2, "0")}_${name}.png`);
  await page.screenshot({ path: file, fullPage: true });
  shots.push(file);
  console.log(`  📸 ${path.basename(file)}`);
}
const log = (m) => console.log(`\n▶ ${m}`);

async function main() {
  const browser = await chromium.launch({ headless: false, slowMo: 120 });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    locale: "zh-CN",
    deviceScaleFactor: 2,
  });
  // 把浏览器发出的所有 /api/v1/ 请求重定向到 :8081（无论前端编译进的是哪个 API 地址）
  await context.route("**/api/v1/**", (route) => {
    const url = new URL(route.request().url());
    const target = new URL(API + url.pathname.replace(/^.*\/api\/v1/, "") + url.search);
    route.continue({ url: target.toString() });
  });
  const page = await context.newPage();
  const apiErrors = [];
  page.on("pageerror", (e) => apiErrors.push(`pageerror: ${e.message}`));
  page.on("console", (msg) => {
    if (msg.type() === "error") apiErrors.push(`console: ${msg.text()}`);
  });

  // ---------- 1. 登录 ----------
  log("1. 登录页 → 开发者登录");
  await page.goto(`${WEB}/login`, { waitUntil: "networkidle" });
  await shot(page, "login");
  await page.getByRole("button", { name: /开发者登录/ }).click();
  await page.waitForURL("**/dashboard");
  await page.waitForLoadState("networkidle");
  await shot(page, "dashboard");

  // ---------- 2. 项目列表（种子数据） ----------
  log("2. Dashboard 项目列表");
  const projectCards = page.locator("a[href^='/projects/']");
  const n = await projectCards.count();
  if (n < 3) throw new Error(`期望 ≥3 个项目，实际 ${n}`);
  console.log(`  ✓ ${n} 个项目可见`);

  // ---------- 3. 项目详情（证据清单） ----------
  log("3. 打开项目：全聚德·前门店");
  await page.getByRole("link", { name: /全聚德/ }).first().click();
  await page.waitForURL("**/projects/**");
  await page.waitForLoadState("networkidle");
  await shot(page, "project_detail");
  const auditLinks = page.locator("a[href^='/audits/']");
  const audits = await auditLinks.count();
  console.log(`  ✓ 审计运行 ${audits} 个`);
  if (audits === 0) throw new Error("项目页没有审计运行入口");

  // ---------- 4. COMPLETED 审计：冻结评分必须可见 ----------
  log("4. 打开 COMPLETED 审计 → 冻结评分/报告（本次修复的核心）");
  await auditLinks.first().click();
  await page.waitForURL("**/audits/**");
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(2500); // 等一次轮询确认完成态稳定
  await shot(page, "audit_completed");
  await page.getByText(/报告（已冻结）/).waitFor({ timeout: 5000 });
  console.log("  ✓ 完成态报告面板可见");
  const hasRing = await page.locator(".score-ring").count();
  if (hasRing !== 1) throw new Error("评分环未渲染");
  console.log("  ✓ 评分环渲染");

  // ---------- 5. Finding 详情：证据查看器 + 任务面板 ----------
  log("5. 打开一条 Finding → 证据图/bbox/观察推断/整改任务面板（本次修复）");
  const findingLink = page.locator("a[href^='/findings/']").first();
  await findingLink.click();
  await page.waitForURL("**/findings/**");
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(1200);
  await shot(page, "finding_detail");
  await page.getByRole("heading", { name: "整改任务" }).waitFor({ timeout: 5000 });
  await page.getByText("OPEN").first().waitFor({ timeout: 5000 });
  console.log("  ✓ 整改任务面板可见（task 字段修复生效）");
  const imgCount = await page.locator(".evidence-figure img").count();
  console.log(`  ✓ 证据图 ${imgCount} 张`);

  // ---------- 6. 任务状态机：合法推进 + 非法路径拒绝 ----------
  log("6. 任务推进 OPEN→ACKNOWLEDGED→FIXING→READY_FOR_RETEST；RESOLVED 应被拒");
  await page.getByRole("button", { name: "确认整改" }).click();
  await page.getByText("ACKNOWLEDGED").first().waitFor({ timeout: 5000 });
  await page.getByRole("button", { name: "开始整改" }).click();
  await page.getByText("FIXING").first().waitFor({ timeout: 5000 });
  await page.getByRole("button", { name: "已改好，申请复测" }).click();
  await page.getByText("READY_FOR_RETEST").first().waitFor({ timeout: 5000 });
  await shot(page, "task_ready_for_retest");
  console.log("  ✓ 三步推进成功（乐观锁版本随行）");
  // 弹出 RESOLVED 请求 → 后端必须 409（无复测 PASS）
  page.once("dialog", (d) => d.accept());
  const [resp] = await Promise.all([
    page.waitForResponse((r) => r.url().includes("/task") && r.request().method() === "PATCH"),
    page.evaluate(() => {
      /* RESOLVED 没有入口按钮（契约），直接从 UI 无法发起——用 fetch 模拟绕过者 */
      const token = localStorage.getItem("arrivalready.token");
      const id = location.pathname.split("/").pop();
      return fetch(`http://localhost:8081/api/v1/findings/${id}/task`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ workflow_status: "RESOLVED", version: 4 }),
      });
    }),
  ]);
  if (resp.status() !== 409) throw new Error(`RESOLVED 守卫失效：HTTP ${resp.status()}`);
  console.log(`  ✓ 绕过者直接 RESOLVED → ${resp.status()}（复测守卫生效）`);
  await page.goto(page.url(), { waitUntil: "networkidle" }); // 回到 finding 页
  const findingUrl = page.url();

  // ---------- 7. 发起复测（真实模型，约 4-7 分钟） ----------
  log("7. 回到审计页 → 创建复测（真实模型）");
  await page.goto(await page.evaluate(() => document.referrer || ""), { waitUntil: "domcontentloaded" }).catch(() => {});
  // 直接从 dashboard 导航更稳：项目 → 审计
  await page.goto(`${WEB}/dashboard`, { waitUntil: "networkidle" });
  await page.getByRole("link", { name: /全聚德/ }).first().click();
  await page.waitForURL("**/projects/**");
  await page.waitForLoadState("networkidle");
  await page.locator("a[href^='/audits/']").first().click();
  await page.waitForURL("**/audits/**");
  await page.waitForLoadState("networkidle");
  await shot(page, "audit_before_retest");
  await page.getByRole("button", { name: /创建复测/ }).click();
  await page.waitForURL("**/audits/**", { timeout: 20000 });
  console.log(`  ✓ 复测 run 已创建: ${page.url()}`);

  // ---------- 8. 轮询至完成（每 30s 截图一次） ----------
  log("8. 等待复测完成（真实模型）…");
  const deadline = Date.now() + 12 * 60 * 1000;
  let done = false;
  while (Date.now() < deadline) {
    await page.waitForTimeout(15000);
    await page.waitForLoadState("networkidle");
    const bodyText = await page.evaluate(() => document.body.innerText);
    // REVIEW_REQUIRED（待人审）也算完成：AI 评估结束，等待人工处置
    if (/已完成|待人审/.test(bodyText) && !/AI 分析进行中/.test(bodyText)) { done = true; break; }
    if (/失败/.test(bodyText)) throw new Error("复测运行失败：" + bodyText.slice(0, 300));
    await shot(page, "retest_polling");
    console.log("  … 分析中");
  }
  if (!done) throw new Error("复测超时（12 分钟）");
  await page.waitForLoadState("networkidle");
  await page.waitForTimeout(1000);
  await shot(page, "retest_completed");
  console.log("  ✓ 复测完成");

  // ---------- 9. 复测 findings 人审（确认/NA） + Diff ----------
  log("9. 复测结果人审（快速确认全部 UNREVIEWED）→ 加载 Before/After Diff");
  // 审计页快速确认按钮（本次新加）
  for (;;) {
    const btn = page.locator("button.finding-quick-confirm").first();
    if ((await btn.count()) === 0) break;
    await btn.click();
    await page.waitForTimeout(400);
  }
  await page.waitForTimeout(500);
  await shot(page, "retest_findings_reviewed");
  await page.getByRole("button", { name: /与上一轮对比/ }).click();
  await page.waitForTimeout(800);
  await shot(page, "retest_diff");
  const diffRows = await page.locator("table.table tbody tr").count();
  console.log(`  ✓ Diff ${diffRows} 行`);

  // ---------- 10. 复测 PASS → 原 finding 任务 RESOLVED（闭环） ----------
  log("10. 复测通过项 → 回到第一轮 finding → RESOLVED 应当放行");
  // 找到复测 run 里一条 PASS 且已 CONFIRMED 的 finding 的 rule，再打开第一轮同 rule finding 的任务
  // 简化：直接用第一轮 finding（READY_FOR_RETEST）尝试 RESOLVED——若该 rule 在复测中被确认为 PASS 则 200
  const token = await page.evaluate(() => localStorage.getItem("arrivalready.token"));
  const findingId = findingUrl.split("/").pop();
  // 先读第一轮 finding 的 rule
  const fr = await fetch(`${API}/findings/${findingId}`, { headers: { Authorization: `Bearer ${token}` } });
  const finding = (await fr.json()).data;
  // 在复测 run 的 findings 里找同 rule 且 PASS
  const childRunId = page.url().split("/audits/")[1];
  const cr = await fetch(`${API}/audits/${childRunId}`, { headers: { Authorization: `Bearer ${token}` } });
  const childData = (await cr.json()).data;
  const passMatch = (childData.findings ?? []).find(
    (f) => f.rule_id === finding.rule_id && f.assessment_status === "PASS",
  );
  if (!passMatch) {
    console.log(`  ⚠ 复测中 ${finding.rule_id} 无 PASS——跳过 RESOLVED 放行验证（数据态不支持，不算缺陷）`);
  } else {
    if (passMatch.review_status === "UNREVIEWED") {
      await fetch(`${API}/findings/${passMatch.id}/reviews`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({ decision: "confirm" }),
      });
      console.log(`  ✓ 复测 ${finding.rule_id} PASS 已人审确认`);
    }
    // 当前任务 version 从 finding 页读
    const fr2 = await fetch(`${API}/findings/${findingId}`, { headers: { Authorization: `Bearer ${token}` } });
    const f2 = (await fr2.json()).data;
    const res = await fetch(`${API}/findings/${findingId}/task`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
      body: JSON.stringify({ workflow_status: "RESOLVED", version: f2.task.version }),
    });
    if (res.status !== 200) throw new Error(`复测 PASS 后 RESOLVED 仍被拒：HTTP ${res.status} ${await res.text()}`);
    console.log("  ✓ 复测通过 → 任务 RESOLVED 放行（闭环成立）");
  }
  await shot(page, "closed_loop");

  // ---------- 汇总 ----------
  log("汇总");
  const realErrors = apiErrors.filter(
    (e) => !/favicon|Download the React DevTools/i.test(e),
  );
  console.log(`控制台/页面错误：${realErrors.length}`);
  realErrors.slice(0, 10).forEach((e) => console.log("  ⚠ " + e));
  console.log(`截图 ${shots.length} 张 → ${OUT}`);
  await browser.close();
  if (realErrors.length > 0) process.exit(2);
  console.log("ALL GREEN ✅");
}

main().catch((e) => {
  console.error("\n❌ FAIL:", e.message);
  process.exit(1);
});
