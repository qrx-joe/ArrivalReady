# 演示指南 ｜ Arrival Ready 迎客验收

> 面向评委与演示者。视频是兜底，现场演示是主线——两者数据同源（同一套种子数据与真实模型链路）。

## 1. 演示视频

- 文件：[arrivalready_demo.mp4](./arrivalready_demo.mp4)（144 秒，1920×1080，中文配音 + 硬字幕）
- 字幕文件：[arrivalready_demo.srt](./arrivalready_demo.srt)（外挂字幕与硬字幕同源）

### 分镜（10 幕）

| # | 时长 | 画面 | 讲述 |
|---|---|---|---|
| 1 | 11s | 开场卡 | 问题：看不懂的菜单、扫不了的码、问不到的路 |
| 2 | 12s | Dashboard | 定位：证据 → AI 评估 → 人审 → 确定性评分 → 整改 → 复测 |
| 3 | 14s | 项目详情 | 真实店铺（全聚德前门）三份材料，哈希指纹防篡改 |
| 4 | 13s | 发起审计 | 证据清单冻结进 run；AI 对照 IRRS 十条规则逐条判断 |
| 5 | 19s | 冻结报告 | 覆盖率 29%：证据不足就诚实 UNKNOWN，宁缺毋假 |
| 6 | 16s | 证据查看器 | 点开结论看照片与定位框；观察≠推断；AI 建议、人签字 |
| 7 | 19s | 人审 + 任务 | 四操作不可覆盖；任务状态机；复测不通过不给 RESOLVED |
| 8 | 15s | 复测对比 | 新 run 保留旧报告；按规则逐条 Before/After |
| 9 | 15s | 工程底座 | 确定性评分 / 契约先行 / 可恢复任务 / 租户隔离 |
| 10 | 10s | 结尾卡 | 游客还没到，功课先补齐 |

## 2. 现场演示路径（约 8 分钟）

前提（一次性）：Windows + Go 1.27 + Node 24/pnpm + Python 3.13/uv；`services/ai/.env` 已配 `MODEL_API_KEY`（StepFun，PO 已配置）；本机 PostgreSQL 17（或 Docker 恢复后走 compose）。

```powershell
# 1) 临时基础设施（Docker 不可用时的等价路径，端口 54329 / :9000）
#    已在演示机运行；新机器按 services/api/README「本地无 Docker」一节初始化
# 2) 启动全栈（AI 侧自动检出真实模型 key，不再退回 fake）
powershell -File scripts\dev-e2e.ps1
# 3) 种子数据：三家真实店铺 + 正式链路上传（幂等，可重复执行）
python scripts\seed_demo.py
# 4) 打开 http://localhost:3000 → 开发者登录
```

演示动线（与视频一致）：

1. Dashboard → 打开「全聚德·前门店」→ 看证据清单（每份 READY + sha256）；
2. 打开一条 COMPLETED 审计 → 冻结报告（评分环 + 维度条 + 覆盖率；讲「诚实 UNKNOWN」）；
3. 点开 PASS · IRRS-D2-001 → 证据图 + 定位 + 观察/推断（Magic Moment）；
4. 打开一条未人审的审计 → 列表上直接「确认」→ 生成报告（冻结评分）；
5. Finding 页走任务：确认整改 → 开始整改 → 已改好 → 讲「RESOLVED 必须复测通过，系统校验」；
6. 复测 run → 与上一轮对比 → Before/After 表。

## 3. 兜底与注意事项（演示前必读）

- **fake / real 边界**：`dev-e2e.ps1` 只在 `services/ai/.env` 缺 `MODEL_API_KEY` 时才启用离线 fake；现场如果看到观察文本是 "fake observation" 即说明 key 未加载，先查 `.env`。
- **真实模型耗时**：一次审计 2–3 份材料约 4–7 分钟（StepFun 视觉模型，逐规则调用）。现场演示优先用已完成的 run 讲结果，再现场发起一次审计讲「分析中」状态即可，不必等它跑完。
- **数据不可变**：AuditRun 完成后不可变、评分冻结后不改；演示中误操作不需要「改数据」，直接新建 run / 新项目。
- **不要执行**：`docker compose down -v`（数据卷保护）；演示前不要删除已引用的证据（retest 会因输入缺失 422）。
- **离线兜底**：视频已入库（本目录），断网/服务故障时直接播放；字幕文件可另挂。

## 4. 重新生成视频（可选）

```bash
# 前提：全栈运行中（API :8081 与 Web :3001 的隔离栈见脚本内说明）；ffmpeg、edge-tts 在 PATH
node scripts/demo/video/record_segments.mjs   # 浏览器片段（Playwright，token 直达目标页）
node scripts/demo/video/rerecord_diff.mjs     # Before/After 片段
node scripts/demo/video/rerecord_analyzing.mjs# 「分析中」片段（会真实发起一次审计）
node scripts/demo/video/record_cards.mjs      # 开场/工程/结尾动画卡
node scripts/demo/video/build_video.mjs       # TTS 配音 + 对齐 + 拼接 + 烧字幕 + 响度归一
```

分镜与配音文案在 `scripts/demo/video/scenes.json`，改文案后删 `raw/tts_*.mp3` 重跑 build 即可。
