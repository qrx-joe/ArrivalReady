# COMMUNICATING ｜ 人机协作记录

> **用途**：记录「人 × AI」的关键协作过程，保证任何一次会话结束后，下一个会话（或任何一个团队成员）都能无缝接上上下文。
>
> **维护规则**
> 1. 每次重要协作会话追加一条记录：日期、参与方、讨论内容、关键决策、产出物、遗留问题；
> 2. 关键决策同步登记到 §2 决策日志（Decision Log），一条决策一行，**只追加不修改历史**；
> 3. 事后补记的历史条目须标注「（补记）」；
> 4. 只记结论与分歧，不贴大段聊天原文。

---

## 1. 协作会话记录

### S-7 ｜ 2026-09-07 ｜ 执行 B05/B06：认证租户与证据闭环
- **参与方**：Product Owner（乔瑞雪）× ZCode（AI）
- **结果**：
  - B05（PR #7）：OIDC RS256 认证（JWKS 缓存/刷新）、租户从库加载、测试身份三重门控、projects/audit_log 组织隔离、ADR-0003 锁定 golang-migrate（D-015）、cmd/migrate（down-all 守卫）、CI postgres service；
  - B06（PR #8–#10）：evidence 校验闸门（大小/magic bytes/服务端 sha256，失败 QUARANTINED）、幂等 complete（fingerprint 重放/冲突）、项目 CRUD、私有下载签名、软删除占位；CI 的 MinIO 改为宿主进程（容器镜像无 shell 不可诊断，两次实测失败后修正）。
- **验证**：真实 PG17（initdb 临时集群）+ 真实 MinIO Windows 二进制上集成测试全绿（迁移 up→down→up、两组织隔离、上传正常/四种隔离态/跨组织 fail closed）；GitHub Actions 四作业绿，集成测试在 runner 上实跑。
- **产物**：migrations 0001–0004、internal/auth、internal/storage、internal/evidence、internal/api、internal/store、internal/testutil、evals/datasets 登记模板、services/api/README。
- **遗留问题**：B07（可恢复 job）未开始——草稿因质量不达标被主动撤销，留待专注会话；B08 起阻塞于 D-007 凭据、ADR-0002 审定、真实素材；D-006～D-009、D-011/D-012 仍待 PO；本机 Docker Desktop 引擎故障未修复（T-003 compose 探活受阻）。

### S-6 ｜ 2026-09-07 ｜ 执行 B02/B03/B04：规则、契约与工程基线
- **参与方**：Product Owner（乔瑞雪）× ZCode（AI）
- **请求**：继续执行方案，直至外部输入阻塞；保持「一步一审一提交」并推送 GitHub。
- **结果**：
  - B02（PR #3）：IRRS 0.1.0 首批 10 条规则草稿（D1–D7，draft）、规则 Schema、评分语义 ADR-0002（草案）、6 个 fixture 与离线校验脚本；
  - B03（PR #4）：OpenAPI 3.1 全量 13 端点、JSON Schema 四件套（含归一化 bbox locator 与 provider envelope）、状态迁移表（四轴分离）、15 个共享 fixture；
  - B04（PR #5）：Go/FastAPI/Next.js 三端空壳 + liveness/readiness、compose、Makefile 与 dev.ps1 等价入口、离线 CI 与 PR 模板；版本经官方渠道复核（Next 16.3.3 / Go 1.27.1 / React 19.2.8）。
- **验证**：三端 lint/type/test/build 与契约校验全部在本机实际跑通（gofmt/vet/go test、ruff/mypy/pytest、biome/tsc/vitest/build、fixtures 21/21）；`docker compose up` 全栈探活**受阻**——本机 Docker 引擎持续 500（进程在、管道无响应），待 PO 重启后补做。
- **产物**：standards/irrs/0.1.0/、contracts/、services/api、services/ai、apps/web、infra 配置、.github/；T-002 完成、T-003 受阻项登记。
- **遗留问题**：D-006～D-009、D-011/D-012 仍待 PO；Docker 引擎恢复后补 T-003 验证；MinIO digest pin 待 compose 首跑；下一批 B05（身份/组织隔离）依赖 T-003 数据库可用。

### S-5 ｜ 2026-09-07 ｜ 执行 B01：范围对齐与文档契约修正
- **参与方**：Product Owner（乔瑞雪）× ZCode（AI）
- **请求**：审查现有文档后开始执行方案，按「执行一步、审查一次、提交一次」推进，并推送 GitHub（qrx-joe/ArrivalReady）。
- **结果**：完成 B01——建立[需求矩阵](docs/requirements-matrix.md)（P0-1～7、US-01～04 → 批次与验收证据）；处理 R-01/R-02/R-09/R-13/R-14/R-15；新增 [ADR-0001](docs/adr/0001-canonical-repository-layout.md) 与 ADR 索引；建立根 [AGENTS.md](AGENTS.md)；同步 PRD/TECH_SPEC/工程规范/TODO/TODO_NEXT/README。
- **产物**：docs/requirements-matrix.md、docs/adr/（0001 + 索引）、AGENTS.md 及六处文档同步；决策 D-010～D-014。审查/方案批次（docs/04、05）先经 PR #1 合入 main，B01 经 PR #2 合入。
- **验证边界**：仅文档层核对（每个 P0 有批次、Slice 3 裁剪口径唯一、暂定/已定可区分、相对链接有效）；未运行应用、未调用模型、未核验外部版本、无 CI。
- **遗留问题**：D-006～D-009 仍暂定；D-011/D-012 草案待 PO 审定；下一批 B02（IRRS 规则草稿与评分样例，AI 起草、PO 审定）。

### S-4 ｜ 2026-09-07 ｜ 文档审查与分批执行方案
- **参与方**：Product Owner × Codex（AI）
- **请求**：审查现有文档，生成细致的分步骤、分批提交、便于回滚的执行方案。
- **结果**：检查 8 份文档与仓库基线，记录 R-01～R-16；方案拆为 B01～B16，补充评分语义草案、阶段门禁、逐批提交、数据库与异步任务回滚约束。
- **产物**：[文档审查](docs/04_DOCUMENT_REVIEW_2026-09-07.md)、[详细执行方案](docs/05_EXECUTION_PLAN_v0.1.md)；第一批提交 `b1ae1ac`，第二批同步文档导航与本记录。
- **验证边界**：只做文档与 Git 检查；没有应用运行、真实模型、外部版本核验、部署或 CI 结果。
- **遗留问题**：D-006～D-009 继续暂定，R-01～R-16 待对应批次关闭，全部 B 批次未开始；下次从 B01 开始，不把方案草案算作 T-001 定稿。

### S-3 ｜ 2026-09-07 ｜ 文档体系建立与执行方案讨论
- **参与方**：Product Owner（乔瑞雪）× ZCode（AI）
- **讨论内容**：
  - 审阅并入库 PRD v0.1 与 TECH SPEC v0.1（来自 D:\EdgeDownload）；
  - 确定 5 份过程文档：TODO / TODO_NEXT / COMMUNICATING / ADVICE / 工程规范；
  - 仓库初始化，文档分步骤提交（Conventional Commits）；
  - 讨论执行方案四项关键决策：比赛时间线、模型供应商、团队配置、验证与开发的并行/串行。
- **产出物**：仓库 docs 基线；TODO.md；TODO_NEXT.md；COMMUNICATING.md；ADVICE.md；docs/03 工程规范 v0.1
- **遗留问题**：D-006 ～ D-009 四项决策（结论以决策日志为准）

### S-2 ｜ 2026-09-07（补记）｜ 技术规范制定
- **参与方**：Product Owner × AI
- **讨论内容**：在 PRD 基础上制定技术规范；确立「模块化单体、契约先行、确定性内核 + 概率性边缘」三原则；确定 Next.js + Go + Python + PostgreSQL 技术栈与 48 小时 MVP 技术切片。
- **产出物**：docs/02_ArrivalReady_TECH_SPEC_v0.1.md
- **遗留问题**：迁移工具 golang-migrate vs Atlas 未锁定（启动后二选一）；Python 3.13 → 3.14 升级待 AI SDK 兼容性验证

### S-1 ｜ 2026-09-07（补记）｜ 机会定义与 PRD
- **参与方**：Product Owner × AI
- **讨论内容**：把「国际游客 AI 导游」方向重构为**供给侧接待准备度验收**；引入 option-skill 的 E0–E4 证据阶梯；定义 IRRS 标准框架、MVP P0/P1/P2 范围与 Kill Criteria。
- **产出物**：docs/01_ArrivalReady_PRD_v0.1.md
- **遗留问题**：E2 用户验证未开始；IRRS 只有维度框架、规则内容未编写（已转 TODO T-004）

---

## 2. 决策日志（Decision Log）

| ID | 日期 | 决策 | 状态 | 依据 / 备注 |
|---|---|---|---|---|
| D-001 | 2026-09-07 | 项目定位：供给侧接待准备度验收，不做游客侧导游/翻译 | ✅ 已定 | PRD §1 |
| D-002 | 2026-09-07 | 采用证据阶梯（E0–E4）管理推进节奏，未达 E2 不扩大投入 | ✅ 已定 | PRD §2 / §22 |
| D-003 | 2026-09-07 | MVP 技术栈：Next.js + Go API + Python AI Service + PostgreSQL | ✅ 已定 | TECH_SPEC §3 / §38 |
| D-004 | 2026-09-07 | AI 编排不用 LangChain 做业务骨架，Pipeline 写普通 domain services | ✅ 已定 | TECH_SPEC §3.3 |
| D-005 | 2026-09-07 | 建立五份过程文档体系并分步骤提交 | ✅ 已定 | S-3 |
| D-006 | 2026-09-07 | **比赛时间线**：暂按 2 周排期 | 🟡 暂定（AI 代拟，待 PO 复核） | 赛事文件确认后立即校准；若 < 1 周则砍 Slice 3 |
| D-007 | 2026-09-07 | **模型供应商**：暂定火山方舟·豆包（1 主）+ Qwen-VL（1 备） | 🟡 暂定（AI 代拟，待 PO 复核） | 依据：视觉+结构化输出成熟、大陆网络稳、环境已有 Ark 工具链 |
| D-008 | 2026-09-07 | **团队人力**：暂按「1 人 + AI」保守排期 | 🟡 暂定（AI 代拟，待 PO 复核） | 实际 ≥ 2 人时 P1 可提前拉入 |
| D-009 | 2026-09-07 | **验证与开发并行**：开发 Slice 1–3，PO 同期完成 3–5 个访谈 | 🟡 暂定（AI 代拟，待 PO 复核） | ADVICE A-5 |
| D-010 | 2026-09-07 | **目录唯一来源**：契约归 `contracts/`、规则归 `standards/irrs/<semver>/`、评测归 `evals/`、Prompt 归 `services/ai/prompts/`；TECH_SPEC §8.1 已同步 | ✅ 已定（工程约定） | R-09；[ADR-0001](docs/adr/0001-canonical-repository-layout.md) |
| D-011 | 2026-09-07 | **范围分层与证据格式分期**：完整 MVP 必含复测（P0-7）；时间不足先砍 P1 与包装，必要时整体降级为「技术验证切片」并显式声明；图片先行、PDF/文本=B13、URL 登记与自动抓取分离（B14）；任何 P0 裁剪须改 PRD 并 supersedes 本条 | 🟡 草案待 PO | R-01/R-02；supersedes TODO「不足一周砍 Slice 3」裁剪规则 |
| D-012 | 2026-09-07 | **交互与语言策略**：手机端上传/人审/任务可用优先，运营分析桌面优先；报告语言与目标访客语言分离（草案：评估字段英文、报告展示中文、证据保留原文） | 🟡 草案待 PO | R-13；ADVICE G-8 |
| D-013 | 2026-09-07 | **分批 PR 约定**：一批一短分支（`codex/<topic>`）一 PR；squash 只限本批；Windows 前提与本地入口记录于 AGENTS.md | ✅ 已定（工程约定） | R-15；docs/03 §8 |
| D-014 | 2026-09-07 | **决策日志 supersedes 机制**：确认/变更一律追加新行并注明 supersedes 旧 ID，历史行不改；废除「直接修改旧决策状态」的做法 | ✅ 已定（工程约定） | R-14；本文档维护规则 2 |
| D-015 | 2026-09-07 | **迁移工具锁定 golang-migrate**：SQL 成对文件 + Go 库嵌入测试 + `cmd/migrate`（down-all 带守卫）；关闭 S-2 二选一遗留 | ✅ 已定（工程约定） | TECH_SPEC §3.2；ADR-0003 |

> 注：D-006 ～ D-009 为 AI 代拟的暂定项（讨论发起后未获 PO 回复），D-011/D-012 为 B01 草案——以上均**不是已确认决策**。确认或变更时**不要修改历史行**：按 D-014 追加新行并注明 `supersedes D-0XX`。

---

## 3. 协作约定（Working Agreements）

1. **文档即契约**：改动 PRD / TECH_SPEC 的范围或决策，必须同步更新决策日志；
2. **假设不冒充事实**：所有市场 / 用户结论必须带证据来源（PRD §28）；
3. **AI 产出先入草稿**：AI 起草的规则、文档、代码，经 Product Owner（或对应 Lead）审定后才算数；
4. **会话收尾三件事**：更新 TODO 勾选、决策日志、遗留问题；
5. **AI 生成代码默认遵守** docs/03 §6 注释与文档规范，注释解释「为什么」而不是翻译代码；
6. **一步一提交**：每个完整的最小变更单元一次 Conventional Commit，禁止攒大提交。
