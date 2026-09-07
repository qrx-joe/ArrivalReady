/**
 * 录制开场 / 工程底座 / 结尾三张动画卡片（HTML+CSS，Playwright 录屏）。
 * 动画时长在页面里写死，最后由 ffmpeg 统一裁齐。
 */
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const webRequire = createRequire(new URL("../../../apps/web/package.json", import.meta.url));
const { chromium } = webRequire("@playwright/test");

const HERE = path.dirname(fileURLToPath(import.meta.url));
const RAW = path.join(HERE, "raw");
fs.mkdirSync(RAW, { recursive: true });

const css = `
  * { margin:0; padding:0; box-sizing:border-box; }
  body {
    width:100vw; height:100vh; overflow:hidden;
    background:#0c1222; color:#f5f7fb;
    font-family:"Microsoft YaHei","PingFang SC",sans-serif;
    display:flex; align-items:center; justify-content:center;
  }
  .stage { width:1200px; }
  .kicker { font-size:26px; letter-spacing:.35em; color:#8ea3c8; opacity:0; animation:fade .9s ease .3s forwards; }
  .line { font-size:64px; font-weight:700; letter-spacing:.02em; margin-top:34px; opacity:0; transform:translateY(18px); animation:up .9s cubic-bezier(.2,.7,.2,1) forwards; }
  .brandrow { margin-top:84px; display:flex; align-items:center; gap:18px; opacity:0; animation:fade 1s ease forwards; }
  .logo { width:46px; height:46px; border-radius:12px; background:#2f6bff; display:flex; align-items:center; justify-content:center; font-size:24px; font-weight:800; }
  .brand { font-size:30px; font-weight:700; }
  .sub { color:#8ea3c8; font-size:20px; margin-left:10px; }
  .d1{animation-delay:1.4s} .d2{animation-delay:3.2s} .d3{animation-delay:4.9s} .d4{animation-delay:6.6s}
  .chips { margin-top:70px; display:grid; gap:26px; }
  .chip { display:flex; gap:22px; align-items:baseline; opacity:0; transform:translateY(16px); animation:up .8s cubic-bezier(.2,.7,.2,1) forwards; }
  .chip b { font-size:34px; min-width:340px; font-weight:700; }
  .chip span { font-size:24px; color:#9fb2d4; }
  .c1{animation-delay:1.2s} .c2{animation-delay:3.4s} .c3{animation-delay:5.6s} .c4{animation-delay:7.8s}
  .center { text-align:center; }
  .center .brandrow { justify-content:center; margin-top:70px; }
  .big { font-size:76px; margin-top:40px; }
  .flow { margin-top:60px; color:#8ea3c8; font-size:24px; letter-spacing:.08em; opacity:0; animation:fade 1.2s ease 3.4s forwards; }
  @keyframes fade { to { opacity:1; } }
  @keyframes up { to { opacity:1; transform:none; } }
`;

const open = `
<div class="stage">
  <div class="kicker">每 一 位 国 际 访 客</div>
  <div class="line d1">看不懂的菜单</div>
  <div class="line d2">扫不了的码</div>
  <div class="line d3">问不到的路</div>
  <div class="brandrow d4"><div class="logo">迎</div><div class="brand">Arrival Ready</div><div class="sub">迎客验收 · 演示</div></div>
</div>`;

const engineering = `
<div class="stage">
  <div class="kicker">工 程 底 座</div>
  <div class="chips">
    <div class="chip c1"><b>确定性评分</b><span>总分只在 Go 侧按标准版本计算，模型碰不到</span></div>
    <div class="chip c2"><b>契约先行</b><span>OpenAPI + JSON Schema，跨语言边界不改口</span></div>
    <div class="chip c3"><b>可恢复任务</b><span>SKIP LOCKED 领取 · 租约 · 崩溃后接着跑</span></div>
    <div class="chip c4"><b>租户隔离</b><span>跨组织访问一律 fail closed</span></div>
  </div>
</div>`;

const close = `
<div class="stage center">
  <div class="kicker">Arrival Ready · 迎客验收</div>
  <div class="line big d1">游客还没到，功课先补齐。</div>
  <div class="flow">Evidence → AI 评估 → 人审 → 确定性评分 → 整改 → 复测</div>
  <div class="brandrow d2"><div class="logo">迎</div><div class="brand">Arrival Ready</div></div>
</div>`;

const html = (body, extraDelay = 0) =>
  `<!doctype html><html><head><meta charset="utf-8"><style>${css}</style></head><body>${body}</body></html>`;

async function shoot(browser, name, body, holdMs) {
  const ctx = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    deviceScaleFactor: 1,
    recordVideo: { dir: RAW, size: { width: 1920, height: 1080 } },
  });
  const page = await ctx.newPage();
  await page.setContent(html(body), { waitUntil: "load" });
  await page.waitForTimeout(holdMs);
  const video = page.video();
  await ctx.close();
  fs.renameSync(await video.path(), path.join(RAW, name));
  console.log(`  🎬 ${name}`);
}

async function main() {
  const browser = await chromium.launch({ headless: true });
  await shoot(browser, "card_open.webm", open, 11500);
  await shoot(browser, "card_engineering.webm", engineering, 12500);
  await shoot(browser, "card_close.webm", close, 9500);
  await browser.close();
  console.log("cards done ✅");
}
main().catch((e) => {
  console.error("❌", e);
  process.exit(1);
});
