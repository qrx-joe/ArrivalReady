# Arrival Ready 详细执行方案 v0.1

> 日期：2026-09-07；依据：docs/01–03、现有任务清单与 [文档审查记录](04_DOCUMENT_REVIEW_2026-09-07.md)。
> 状态：可供逐批推进的方案草案；产品范围及 D-006～D-009 仍待确认。本文新增的公式、阈值与时间估算均为建议值，不能当作已批准规范。
> 本次交付范围：审查、编写执行方案、文档分批本地提交。以下 B01–B16 是后续实现批次，目前均未执行；本次不安装依赖、不调用模型、不部署、不推送。

## 1. 完成标准与范围分层

最终演示应在同一项目中完成：创建项目 → 上传材料 → AI 按指定规则产生带证据的 Finding → 人工确认 → 确定性报告 → 创建整改任务 → 上传新证据 → 新 AuditRun → Before/After。旧 run 的判断、证据关联与分数不能被新操作覆盖。

三个验收层级分别记录，不能互相替代：

| 层级 | 必须可验证的结果 | 不能宣称的结果 |
|---|---|---|
| 工程切片 | 本地单图流程、失败态、证据定位；自制 fixture 或明确标记的测试 provider | 完整 MVP、真实模型质量、真实商户价值 |
| 核心闭环 | 真实模型、人审评分、整改、新 run、可比较 Diff；20 例评测与安全门禁 | 所有原始 P0 格式已支持，除非 B13–B14 同样验收 |
| 比赛 MVP | 核心闭环 + 已确认的 P0 范围 + 授权真实案例 + PRD §26 对应证据 + 演示包 | 产品需求已普遍成立、试点或生产验收通过 |

原 PRD 的 PDF/URL/文本 P0 要求继续保留。建议将“URL 仅登记来源”与“服务端抓取 URL”分开；如果 PO 接受将自动抓取延期，必须修改 PRD，不靠 TODO 静默降级。没有该决定时，B14 仍是完整 P0 的组成部分。

## 2. 推进原则、人员与待定事项

以“1 人 + AI”作为估算假设，不虚构多人并行产能。AI 负责实现和验证材料，PO 负责真实事件访谈、规则语义/权重审定与产品范围判断；角色标签不表示已有专职人员。

| 待定项 | 当前依据 | 最晚影响点 | 确认前可做 |
|---|---|---|---|
| deadline、评审细则、访问方式 | D-006 暂按两周；实际日期未知 | B01 范围冻结、B16 演示安排 | 文档、规则草稿、契约设计 |
| 供应商、具体视觉模型、可用额度 | D-007 豆包主/Qwen 备暂定，未证明凭据可用 | B08 真实调用 | adapter 接口与离线 fixture |
| 人员投入 | D-008 暂按 1 人 + AI | 各批估算复校 | 按单执行通道排依赖 |
| 访谈与开发并行方式 | D-009 暂定 | 真实案例与 MVP 验收 | 模板、招募提纲、自制 fixture |
| P0 证据范围与部署目标 | R-01/R-02/R-08 | B01/B05/B14 | 保留原范围，隔离本地开发 |
| 评分/适用性规则与报告语言 | R-04/R-13 | B02/B10 | 提供可手算草案供审定 |

这些事项只阻塞依赖它们的工作，不要求为每个文件反复确认。既有 StepFun 小规模测试授权仅在实际选择 StepFun 且已有配置适用于本任务时使用，不自动把 D-007 改成 StepFun，也不推定其他供应商费用授权。

## 3. 总体顺序与阶段门禁

```text
B01 范围/决策 → B02 规则与评分语义 → B03 契约 → B04 工程环境
  → B05 身份/组织隔离 → B06 项目与上传 → B07 异步任务
  → B08 模型与早期评测 → B09 单图竖切
  → B10 人审评分报告 → B11 整改 → B12 复测对比
  → B13 PDF/文本 → B14 URL（或经确认延期）
  → B15 完整评测与可靠性验收 → B16 演示与交接

并行人工轨道：访谈 → 规则审定 → 授权素材 → 人工 Audit → 整改/复测案例
```

每批按“读依赖 → 小步实现 → 定向验证 → 审查 diff → 提交 → 更新记录”执行。先建规则和契约，再由契约生成或校验实现；不先分别开发三套独立类型。

| Gate | 所属批次 | 放行条件 |
|---|---|---|
| G0 可编码 | B01–B04 | 阻碍当前切片的范围/语义已确定；fixture、契约、运行命令可复现 |
| G1 可接真实素材 | B05–B06 | 登录/组织隔离、私有文件、上传检查与授权/脱敏记录有效 |
| G2 Slice 1 | B07–B09 | 真实模型单图端到端；点开结论可看证据；失败/取消/重试不污染数据 |
| G3 Slice 2 | B10 | 人审后算分；绑定版本与覆盖率可解释；历史报告不可变 |
| G4 Slice 3 | B11–B12 | 至少一次 FAIL→整改→新证据→人工确认 PASS；旧 run 不变；缺证据不误报解决 |
| G5 MVP | B13–B16 | 已确认 P0、20 例评测、安全关键路径、真实案例、回滚演练及演示包通过 |

## 4. 逐批实施清单

下列路径是计划产物，目前并不存在。每批都含自己的必要测试与文档更新，不能等 B15 才开始补测试。B01 是需要形成具体决策的批次；涉及 PO 判断的项未确认时只提交草案，不假装已冻结。

### B01 — 冻结范围、修正文档冲突

- **依赖 / Owner**：审查报告；PO + AI。预计 0.5–1 人日，不含等待回复。
- **步骤**：① 建需求矩阵，逐项列 PRD P0-1～7、US-01～04、批次和验收证据；② 处理 R-01/R-02/R-09/R-13/R-14/R-15；③ 在决策日志追加 supersedes 记录，不覆盖 D-006～009；④ 确定目录唯一来源、MVP 范围、语言策略和分批 PR 约定；⑤ 同步原文版本声明、TODO 与 TODO_NEXT。
- **产物**：`docs/requirements-matrix.md`、必要 ADR、项目根 `AGENTS.md`、同步后的 docs/01–03 与任务清单。首次创建 ADR 索引。
- **verify**：每个 P0 都有批次；没有互相矛盾的 Slice 3 裁剪规则；暂定/已定可区分；链接有效。
- **提交**：`docs(plan): reconcile scope and execution contracts`。
- **回滚**：仅文档可 revert；若后续代码依赖新契约，先处理后续批次，不能只退规范造成代码与文档不一致。

### B02 — 编写规则、适用性与评分样例

- **依赖 / Owner**：B01 当前范围；AI 起草、PO 审定。预计 1–2 人日，30–50 条完整版还需人工复核时间。
- **步骤**：① 先写 7–10 条餐饮规则草稿覆盖 D1–D7；② 补场景/目标语言/证据适用性、severity、weight、rationale、正反例和人审标记；③ 区分“不适用”与“缺证据”；④ 写评分 ADR 与手算样例；⑤ 分批扩到 30–50 条并用真实材料人工走查，未审定保持 draft。
- **产物**：`standards/irrs/0.1.0/rules.yaml`、规则 Schema、`docs/adr/0002-scoring-and-review.md`（编号以实际索引为准）、初始 fixture。
- **verify**：规则 code 唯一、维度合法、权重非负、必需证据可检查；支付标识照片不能证明实际支付成功；所有评估不依赖图片可见信息之外的臆测。
- **提交**：`feat(standards): add draft IRRS rules and scoring fixtures`；扩充与发布标准作为本批后续独立小提交，不混入 UI。
- **回滚**：草稿可 revert；已发布标准永不原地改写，退 active 指针仅影响新 run，保留历史版本文件与记录。

### B03 — 固定跨语言契约与领域语义

- **依赖 / Owner**：B02 草稿语义；AI。预计 1–1.5 人日。
- **步骤**：① 定义 Project/Evidence/Audit/Finding/Review/Task 及错误 envelope；② 定义 normalized evidence、assessment、provider response、job payload Schema；③ 分开 assessment_status、review_status、workflow_status；④ 明确 run 输入快照、人工编辑内容与版本字段；⑤ 约定图片 locator 原图归一化坐标、范围与原点；⑥ 补合法/非法共享 fixture。
- **产物**：`contracts/openapi/arrivalready.yaml`、`contracts/json-schema/`、`contracts/fixtures/`、状态迁移表。主键 UUIDv7；规则业务 code 与数据库 ID 字段明确区分。
- **verify**：拒绝未知规则、跨项目/跨 run 证据、非法 bbox、错误枚举、缺失字段、越界 confidence；Go/Python 校验同一 fixture，若生成器不兼容则验证支持范围并写 ADR，不能静默降级契约。
- **提交**：`feat(contracts): define audit review and evidence schemas`。
- **回滚**：没有消费者时直接 revert；已有消费者先退实现。数据库迁移不与本批混做。

### B04 — 最小运行环境与离线 CI

- **依赖 / Owner**：B03；AI。预计 1–1.5 人日。
- **步骤**：① 检查现有 Windows/Docker/Node/pnpm/Go/uv 条件；② 官方核验版本兼容性并锁定，不沿用未经本次核对的精确版本声明；③ 建 Web/API/AI 三端空壳与 PostgreSQL/MinIO；④ 增加 liveness/readiness，依赖未就绪返回可诊断状态；⑤ 提供 `Makefile` 及 `scripts/dev.ps1` 等价入口；⑥ 建 CI 基线、PR 模板和依赖锁文件。
- **产物**：`apps/web/`、`services/api/`、`services/ai/pyproject.toml` 与 uv lock、compose、scripts、`.env.example`（无密钥）、`.github/workflows/ci.yml`。
- **verify**：干净环境按 README 启动；三端 build/lint/type/test 实际运行；契约正反例可在无模型 key 的 CI 通过；缺配置能明确报错。未实现的测试入口不能用空命令伪装通过。
- **提交**：`build(workspace): bootstrap reproducible local services`；CI 可另一个 `ci: add offline quality gates` 小提交，依赖前者。
- **回滚**：退空壳代码/配置；保留数据库 volume，不执行 `docker compose down -v`。本批不包含外部 CI 平台配置、部署或分支保护的远程修改。

### B05 — 身份、租户与最小审计基础

- **依赖 / Owner**：B04；AI，PO 提供适用身份配置。预计 1–1.5 人日。
- **步骤**：① 建 organization/user 与基础迁移；② 接 OIDC 并验证 issuer/audience/expiry/signature；③ 统一资源归属校验，不信任请求体 organization_id；④ 建 actor/request_id/audit log；⑤ 明确开发测试身份只能在显式本地测试模式使用，真实数据路径禁止绕过。
- **产物**：API auth adapter/domain、迁移与 repository 测试、认证说明和素材登记模板。
- **verify**：两个组织之间不能读取/修改项目或获取他方文件签名；过期/伪造 token 拒绝；日志没有 token/原始材料；AI 服务仅供内部调用。
- **提交**：`feat(auth): enforce organization scoped access`。
- **回滚**：真实数据已进入后不得回退到无鉴权版本；停用入口并向前修复，保留账户、日志与迁移。

### B06 — 项目与图片证据闭环

- **依赖 / Owner**：B05；AI。预计 1–1.5 人日。
- **步骤**：① 项目创建/列表/详情；② upload-url 生成随机 object key；③ 完成上传时校验对象实际大小、Content-Type、magic bytes、hash、扫描状态及归属；④ upload complete 幂等；⑤ 校验成功才设 READY，未完成/隔离对象不能送 AI；⑥ 私有下载签名、删除占位、来源/授权/采集时间记录。
- **产物**：project/evidence API、存储 adapter、迁移、上传 UI、上传/保留策略文档。
- **verify**：正常图、伪装扩展名、超大图、过期签名、重复 complete、他人 object key、未完成上传、扫描失败；清理孤儿对象不得删除已被 run 引用的材料。
- **提交**：`feat(evidence): add validated private image uploads`。
- **回滚**：关闭上传入口；保留元数据和对象。应用回退不等于回删用户材料。

### B07 — 可恢复的 Audit jobs

- **依赖 / Owner**：B06 + B03；AI。预计 1–1.5 人日。
- **步骤**：① 创建 Audit 时绑定规则版本、scope/locale、证据 ID/hash 与输入清单；② 同事务创建 run/job 和幂等记录；③ Go worker 领取 job、释放数据库锁后调用 Python；④ 设置租约、attempt token、有限重试与取消检查；⑤ 结果验证后同事务写 Findings/关联/状态；⑥ 提供状态查询与失败原因。
- **产物**：job migration/worker、Audit API、恢复说明；Python 不直接修改核心业务表。
- **verify**：并发领取不重复生效；worker 领取后崩溃可恢复；重复响应不重复落库；取消后的晚到响应无效；COMPLETED 不可取消或改 FAILED；相同幂等键不同 payload 返回冲突。
- **提交**：`feat(audit): add leased jobs and atomic result persistence`。
- **回滚**：暂停领取并排空/记录在途任务，旧版本不能解释的新 job 保留待处理；不删 job 表，不自动重发可能已计费请求。

### B08 — Provider、Grounding 与早期 Eval

- **依赖 / Owner**：B03/B07、供应商可用性；AI。预计 1–2 人日。
- **步骤**：① 实现 `StructuredModel` 与返回 metadata 的 envelope；② extraction/assessment Prompt 文件版本化；③ 确定性筛规则；④ Schema + 规则/证据/locator grounding 二次检查；⑤ fallback/repair 共用总调用预算；⑥ 首日建立评测 runner 和最少 5 个 fixture，含合法/缺证据/伪造引用/注入文本/供应商失败。
- **安全失败**：材料中的指令视为数据；无效引用不入正式 Finding；缺信息为 UNKNOWN；结构损坏用失败态，不编造 PASS。UI 不展示为已审核结论。
- **产物**：`services/ai/providers/`、`services/ai/prompts/`、domain pipeline、`evals/runners/`、版本/用量报告。
- **verify**：离线 fake adapter 全路径；获授权后真实菜单图到合法 JSON；记录 provider/model/prompt/schema/hash、每次 attempt 的 tokens/latency。费用信息不可得时填 unknown，不填 0。
- **提交**：`feat(ai): add grounded assessment provider and eval runner`；备用 provider 可独立小提交，未接通不得声称 fallback 验收通过。
- **回滚**：退 provider/prompt bundle 配置，仅影响新 run；不重算旧结果。真实调用建议首次最多 6 个 provider 请求（重试/repair/fallback 均计入），连续 2 次失败或额度/鉴权错误停止；该上限不是人民币费用上限。

### B09 — Slice 1 前端证据工作流

- **依赖 / Owner**：B06–B08；AI。预计 1–1.5 人日。
- **步骤**：① 项目创建/详情、图片上传；② 启动 Audit 与状态轮询（结束或离页停止）；③ Finding 列表与 Evidence Viewer；④ 原图/定位/规则/观察与推断/置信度/待人审标识；⑤ loading/empty/error/UNKNOWN；⑥ 手机上传与桌面/手机人审入口。
- **verify**：Playwright 实际走创建→上传→Audit→Finding→打开证据；另跑真实模型冒烟并单独标注；定位随缩放保持；键盘可操作；模型不可用不白屏、不展示缓存为 live。
- **提交**：`feat(web): deliver image audit evidence workflow`。
- **回滚**：回退 Web 到兼容 API 的前一版本；B07/B08 后端可保留。G2 未通过不能标记 T-009 完成。

### B10 — 人工审核、确定性评分与冻结报告

- **依赖 / Owner**：B09、B02 评分语义审定；AI + PO。预计 1–2 人日。
- **步骤**：① 实现 confirm/reject/edit/na；② edit 保留原始候选和修订字段，理由与 actor 可追溯；③ 乐观版本控制，旧页面提交返回冲突；④ 处理全部范围内检查项，不只累计 FAIL Findings；⑤ 确定性评分；⑥ finalize 原子冻结人审、分数、版本与证据快照，完成后只读。
- **verify**：手算样例一致；N/A/UNKNOWN/Info/Reject、零分母、S0/S1 报警、并发 review/finalize、重复 finalize；新 active 标准不改旧分数；未审核不标最终报告。
- **提交**：`feat(review): add deterministic immutable audit reports`。
- **回滚**：保留已完成报告，不逆向重算；关闭新增 finalize 或退兼容应用。评分修复发布新算法/标准版本，历史更正显式生成新报告/run。

### B11 — 整改任务与显式状态机

- **依赖 / Owner**：B10；AI。预计 0.5–1 人日。
- **步骤**：① task 关联历史 Finding，但活动状态不覆盖历史判断；② owner 默认当前用户，不顺带开发团队协作；③ 迁移表覆盖 OPEN/ACKNOWLEDGED/FIXING/READY_FOR_RETEST/RESOLVED/ACCEPTED_RISK/REOPENED；④ 每次迁移记录原因、actor、版本和时间。
- **verify**：非法跳转拒绝；REOPENED 可重新进入整改；ACCEPTED_RISK 必须带理由且不等于 PASS；人工点击“已改好”不能直接证明 RESOLVED。
- **提交**：`feat(tasks): track remediation with audited transitions`。
- **回滚**：关闭任务编辑，保留事件和当前状态；报表保持可读。

### B12 — 新 run 复测与可解释 Diff

- **依赖 / Owner**：B11；AI。预计 1–1.5 人日。
- **步骤**：① Retest 创建 parent_run_id 指向旧 run 的新 run；② 显式选择新证据与保留证据清单，绝不默认替换旧对象；③ 稳定检查键对齐；④ 同标准/范围/profile 比较，否则标不可比较；⑤ 复测经人审后更新独立任务投影；⑥ 展示新增/仍失败/改善/回退/未评估。
- **verify**：FAIL→PASS、FAIL→UNKNOWN、缺项、新 scope、新标准、重复 retest、非父链对比；旧 run 序列化快照/hash 不变；UNKNOWN 或没返回 Finding 不能自动解决任务。
- **提交**：`feat(retest): preserve history and compare reviewed runs`。
- **回滚**：保留新旧 runs 与 parent 关系；禁用新复测，已有报告可读；回退 UI 不倒写任务历史。

### B13 — 补齐 PDF 与文字证据

- **依赖 / Owner**：B12、P0 范围；AI。预计 1–2 人日。
- **步骤**：① 复用 ingestion 接口加文本与 PDF handler；② PDF 页数/解压后大小/超时/隔离，失败进入隔离态；③ 页码定位与文本引用范围；④ 增加相应规则适用性与 fixture；⑤ UI 清楚展示类型限制。
- **verify**：损坏/加密/超大 PDF、多页定位、文本越界引用、解析超时；一个图片案例的历史输出不因新增 handler 改变。
- **提交**：`feat(evidence): support bounded PDF and text ingestion`。
- **回滚**：禁用新增类型入口，保留已入库类型的历史查看能力；不能退到无法读取既有 Evidence 类型的应用版本。

### B14 — URL 范围兑现或正式延期

- **依赖 / Owner**：B01 决策与 B13；AI + PO。预计自动抓取 2–3 人日；仅延期决策不计为功能实现。
- **步骤**：① 若正式延期，同步 PRD/验收矩阵，明确来源登记不抓取；② 若保留自动抓取，独立隔离执行器、出站限制、DNS/IP/重定向逐跳校验、超时与体积限制；③ 保存内容快照/hash/抓取时间，不能用不断变化的 live URL 代替 Evidence；④ 增加抓取失败与用户补传入口。
- **verify**：本地/私网/IPv6 loopback/link-local、metadata、非 HTTP 协议、重定向至内网、DNS rebinding、超大响应、无限跳转全部受控；不能把仅字符串检查等同 SSRF 防护。
- **提交**：`feat(evidence): add isolated URL snapshot ingestion`，或仅文档 `docs(scope): record approved URL ingestion deferral`。
- **回滚**：禁用抓取入口与执行器，保留已采集快照。安全要求未完成时此功能不可启用；延期未获确认时完整 MVP 仍未达成。

### B15 — 完整 Golden Set 与可靠性验收

- **依赖 / Owner**：B08 runner、B12 核心闭环、已确认格式范围；PO 标注、AI 运行。预计 1–2 人日，不含素材等待。
- **步骤**：① 规则扩到经审定目标集；② 20 个真实案例按 8 FAIL/5 WARN/5 PASS/2 UNKNOWN 建立预期；③ 禁止断言与 evidence 引用人工复核；④ 保存模型/Prompt/规则/Schema/数据集/代码版本；⑤ 执行关键 E2E、安全与崩溃恢复；⑥ 做数据库备份恢复和一批应用回退演练。
- **verify**：遵循下文评测门槛；PRD §19/TECH_SPEC §19.4 对应路径都有证据；CI 无 key 仍可跑离线检查，付费评测独立触发，普通代码 PR 不无限调用模型。
- **提交**：`test(mvp): add golden regression and recovery acceptance`。真实原始材料不默认提交仓库；提交脱敏 manifest、hash 与可访问性说明。
- **回滚**：测试/门槛回退不能掩盖已知失败；保留失败报告，相关版本不得标记可交付。

### B16 — Demo 包与交接

- **依赖 / Owner**：G4 与 G5 前置验证；AI + PO。预计 1–2 人日。
- **步骤**：① 按 PRD §25 排练 90 秒流程；② 真实运行生成 Cached Example，显示来源与版本；③ 准备录屏与网络失败提示；④ 写启动、备份、恢复、常见故障、停止 worker 的 runbook；⑤ 核对需求矩阵并冻结功能；⑥ 部署若需要另列具体环境、费用、配置及验证，按授权边界执行。
- **verify**：陌生执行者照 runbook 能启动/恢复；缓存和实时不混淆；所有链接/原图可访问；一个授权案例可从初检追到复测；记录未完成 P0 与未达业务证据，不能以录屏代替验收。
- **提交**：`docs(demo): add verified demo and operating runbook`。
- **回滚**：撤回演示配置或切换上一已验证包；tag 只指向相应门禁通过的 commit，禁止覆盖旧 tag。本批本身不默认授权公开发布。

## 5. 评分与数据契约草案：B02/B03 必须解决的细节

### 5.1 评分建议（审定后才能实现为正式规则）

1. 以 run 冻结的“适用检查项全集”为分母来源；候选规则筛选不能悄悄缩小范围。每个检查项有稳定 key，多个 Evidence/Finding 不重复计权。
2. 建议 PASS=1、WARN=0.5、FAIL=0；UNKNOWN 和未审核项不进入已评估分数；N/A 经人审并留理由后从适用范围排除；Info 不扣分也不增加计分分母。
3. 维度分 `100 × Σ(规则权重 × 状态值) / Σ已评估规则权重`。维度覆盖率 `已评估适用权重 / 全部适用权重`；同时显示条目覆盖数。
4. 建议只有所有适用计分项已审定且不含 UNKNOWN 时才发布最终总分；否则显示“部分评估”与维度分，total_score=null。若希望允许低覆盖率总分，另定阈值并解释风险。
5. 总分按该 run 的维度权重加权；整维 N/A 才可排除并重新归一化，整维缺证据不能按 N/A 处理。全部 N/A/UNKNOWN 时无分数，不能给 0 或 100 冒充已验收。
6. S0/S1 FAIL 独立列为阻断/关键问题，不能被高均分隐藏；严重度不再暗中乘一次权重；置信度不参与算分。
7. Reject 表示候选结论未获认可，不自动转 PASS；要么给人工替代判断，要么保留 UNKNOWN。finalize 前所有人审项都有处置，存在 UNKNOWN 的报告仍明确标为部分评估。
8. 报告冻结评分算法版本与规则包 hash，金额/权重等数值选择稳定精度；规定最后一步取整，避免各端浮点与四舍五入不同。

手算验收例：一个维度三项权重分别 2/1/1，结果 PASS/WARN/FAIL，得分 `100×(2+0.5+0)/4=62.5`、覆盖率 100%；将最后一项改 UNKNOWN，部分分为 `100×2.5/3≈83.33`、覆盖率 75%，总分不发布，不能宣称准备度改善。最后一项若经审定 N/A，则适用权重变 3；这与缺证据是不同决策。

### 5.2 不变量与调用边界

- Web → Go API → jobs/worker → Python → Provider；Go 校验返回后写数据库。模型调用不在持有数据库行锁的事务里等待。
- Python 接收冻结规则与证据清单，只输出候选结果和元数据；不得修改人审、评分、权限或任务状态。
- AuditRun 在 REVIEW_REQUIRED 前可更新执行状态；finalize 时冻结报告。COMPLETED/FAILED/CANCELLED 的后继行为由状态表明确，重跑通常产生新 run。
- 原始 AI assessment、人工有效判断、整改工作流分别存储。完成报告后更正不能直接覆盖其人审记录；历史任务活动独立追踪。
- 发布标准不可变；模型版本不可获得时如实 unknown，至少保留请求模型标识与调用时间，不能承诺第三方模型完全可重放。
- 幂等 key 按组织+操作作用域存储 request fingerprint 和结果；过期策略明确。外部调用最多做到有限重复与内部只生效一次，不承诺 provider 端 exactly-once 计费。
- 租约过期的旧 attempt 不能提交结果；取消不是仅 UI 变色。retest/finalize/upload complete 同样需要竞争条件测试。
- Evidence 删除或到期后显示不可再查看原因，保留允许保留的 hash/元数据；不要把哈希当作原始证据替代品。

## 6. 验证矩阵与模型请求控制

| 变化 | 必跑验证 | 证据保存 |
|---|---|---|
| 文档 | diff whitespace、相对链接、范围与决策状态 | 提交描述、审查记录 |
| 规则/Schema | 合法/非法 fixture、code/引用完整性、评分样例 | 离线测试结果 |
| Go 领域/迁移 | 单测 + 真实临时 PostgreSQL 集成 + migration 空库/升级检查 | 测试报告与迁移版本 |
| 上传/权限 | 两组织隔离、签名与对象校验、失败路径 | 集成测试与必要浏览器结果 |
| UI 工作流 | 类型/build、关键 E2E、手机/桌面与键盘检查 | 实际截图/trace，标环境 |
| 模型/Prompt/规则版本 | 离线结构回归 + 授权真实模型 golden 对比 | 输入/版本/hash、逐 case 结果、请求数/usage |
| 发布候选 | 全部关键路径、备份恢复、应用回退演练 | runbook 执行记录 |

建议初版评测门槛：正式入库结果 Schema 合法率与引用归属校验 100%；20 例中严重的无证据断言为 0；人工标注的 S0/S1 例不得漏检；对已有 baseline 不允许新增关键漏检或新增无证据断言。普通状态匹配率先建议 ≥90%，须在 B15 前由 PO 确定，不能看完结果再降门槛。没有 S0/S1 样本时 Recall 标 N/A，不能报 100%。

原始模型 Schema 成功率与“修复后最终合法率”分开；Grounding 的 ID 合法与语义证据支持率分开。所有指标同时给分子/分母，20 例只提供方向性质量证据；P95 标注样本数，不当作生产 SLO 证明。

请求预算建议：B08 首次最多 6 次 provider 请求；B15 单轮 20 cases，若每 case extraction+assessment 两次，则基础 40 次、全轮硬上限 60 次（含全部重试）。fallback 与 repair 共用此上限；鉴权/额度错误立即停止，连续 2 次传输失败停止并检查。达到上限即保存未完成记录，不自动开第二轮。开始前输出实际计划、单次输出 token 上限、超时与总上限，结束后报告实际调用次数和可得 usage；不能将次数换算为未经核实的人民币费用。

## 7. 提交、PR 与回滚操作规程

### 7.1 提交粒度

- 每个 B 批次是一个可独立评审/回滚的交付单元；较大批次再拆成“契约/兼容迁移”“业务实现与测试”“UI 与 E2E”等小提交，但每个提交都应能解释和验证。
- 使用 `codex/<topic>` 短分支。一个批次一个 PR；若 squash merge，只 squash 本批，不把 B01–B16 合成一个大 PR。超过两天的批次再拆小，依赖写在 PR 描述中。
- 不按文件类型盲目拆提交：新行为和证明该行为的测试通常同提交；不要提交只有数据库破坏而没有兼容路径的中间态。
- 每次只暂存明确路径，检查 `git diff --cached` 与 `git status`，不把素材、密钥、缓存、私人访谈原文带入仓库。
- 本地通过、CI 通过、真实模型、浏览器、部署验证分别登记。未推送没有 CI 运行证据，不写“CI 全绿”。

建议每批交接模板：

```text
批次 / commit / 父基线：
需求与审查问题：
实现与变更路径：
验证命令、结果、实际环境：
数据迁移与旧应用兼容性：
回滚前置条件、操作、回滚后验证：
未完成项 / 下一批依赖：
```

### 7.2 常规 Git 回滚

以下为操作模板，占位 SHA 必须先从实际日志选择；本次不执行回滚。

```powershell
git status --short
git log --oneline -12
git show --stat <待回滚SHA>
git revert <待回滚SHA>
```

有依赖的多个批次，按最新消费者 → 较早提供者逆序 revert，并在每一步运行相应验证。不要机械按批号回退有并行依赖的提交；先查实际图与迁移兼容表。不使用 reset --hard、强推或清空数据库模拟回滚。

### 7.3 数据与运行中的任务

1. **先隔离写入**：关闭相关入口，暂停 worker 领取，记录在途任务和 attempt，不把停机当作任务成功。
2. **核对恢复点**：迁移版本、应用 commit/镜像、数据库备份、对象存储清单与 hash 都要对应；备份必须在隔离环境验证可恢复。
3. **优先只退应用**：新增表/列向后兼容时保留 schema 和数据；旧应用若不兼容新 enum/job payload，不能直接上线旧版本。
4. **有业务数据的破坏性 down 禁止自动执行**：选择向前修复或经授权恢复。空临时测试库可以验证 down/up，但不能因此认定真实库可安全 down。
5. **数据库与对象文件一起考虑**：恢复数据库不能自动恢复文件；迁移回滚不删除用户上传对象；删除/保留策略独立执行。
6. **复核**：权限、旧报告读取、文件访问、历史 hash、队列状态均正确后才恢复写入。恢复结果和失败事实写入 runbook。

## 8. 时间盒与裁剪规则

上述批次逐项粗估合计约 16–27.5 人日，不含访谈等待、模型/身份配置、赛事变化和陌生环境排障；这是工作量估算，不是交付承诺。B02 规则扩充与人工审定、B15 素材整理可能额外占用 PO 时间。

两周暂定窗口不应简单分配成“每天一个大功能”。建议第一检查点在 B09，第二在 B12；每完成一批记录实际耗时并重估剩余量。若两周按 10 个工作日且仅 1 人投入，当前完整范围明显存在超载风险。

时间不足的顺序：先移除未拉入的 P1（Guest/QR/导出/多用户）、精简视觉包装、减少重复规则与非关键自动化；再由 PO 明确 PDF/URL 等 P0 的阶段延期。复测属于产品价值验证，不默认砍掉。只有 48 小时时目标改为技术切片与手工 Audit，不声称完整比赛 MVP。

演示前 48 小时冻结功能是预留建议，前提是确认真实日期；冻结期修复 blocker、核对案例、做恢复演练和录屏。若关键 Gate 失败，交付说明必须写清未完成，不能用缓存掩盖。

## 9. 任务清单映射与下次入口

| 原任务 | 执行批次 / 完成限制 |
|---|---|
| T-001 | B01；文档方案已写不等于四项暂定决策已确认 |
| T-002/T-003 | B04（以及 B01 项目规则）；必须实际跑通环境 |
| T-004 | B02 草稿开始，B15 前完成 30–50 条审定目标 |
| T-005 | 并行人工轨道；至少记录事件/失败环节/代价，依据 PRD §22 作判断 |
| T-006 | B02 自制 fixture 起步，B15 满足真实 20 例；不能提前勾选 |
| T-007 | B03 |
| T-008 | B08；主备供应商各自验证状态单列 |
| T-009 | B05–B09，G2 验收 |
| N-01/N-02 | B10 / B11–B12 |
| N-03 | B08 提前建 runner，B15 完整验收 |
| N-04/N-05 | B05–B07 前置安全/日志；B15 补全验收，OTel 随 B04/B08 逐步贯通 |
| N-06 | B16 |
| N-07 | 真正 P1 继续排队；URL 的 P0 冲突在 B01/B14 显式解决 |

下次从 B01 开始：读取本方案与审查表、确认工作区状态，先形成范围矩阵和具体决策草案；只有遇到不可推断的产品决策才询问。当前全部 B 批次为未开始，不把本次文档提交记为工程能力已经完成。

结构影响：方案覆盖规则/契约、存储与权限、Go 确定性业务、Python 推理、Web 工作流和交付验证六部分；本次实际只修改文档层。既有 PRD/TECH_SPEC/工程规范正文保留，冲突已列待关闭项，后续 B01 必须同步处理。
