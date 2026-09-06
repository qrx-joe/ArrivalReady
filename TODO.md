# TODO ｜ 当前周期任务清单

> **维护规则**
> - 本文件只放「当前周期」（Sprint 0）的任务；完成的任务打勾并移到底部 Archive；
> - 新任务先进 [TODO_NEXT.md](TODO_NEXT.md) 排队，周期开始时再拉入本文件；
> - 每个任务必须有：**产出（Deliverable）**、**完成标准（DoD）**、**负责人（Owner）**；
> - 状态标记：`[ ]` 未开始 ｜ `[~]` 进行中 ｜ `[x]` 已完成 ｜ `[!]` 受阻（须写明阻塞原因与解除人）。

---

## 本周期目标（Sprint 0 Goal）

完成「**文档基线 → 工程基线 → IRRS 规则集 v0 → Slice 1 竖切打通**」，使得：

> **创建项目 → 上传一张图片 → AI 结构化评估 → 产出带 Evidence 引用的 Finding**

可以在本机端到端运行。这是 TECH_SPEC §35 Slice 1 的原样落地，Slice 2/3 不碰。

周期时长：建议 5–7 个工作日，**待决策 D-006（比赛时间线）确认后校准**。

---

## A. 决策与基线

- [ ] **T-001 执行方案定稿**
  - 产出：按确定的「时间线 / 模型供应商 / 团队配置 / 验证顺序」四项决策，更新本文件与 TODO_NEXT.md 的排期；
  - DoD：四项决策写入 [COMMUNICATING.md](COMMUNICATING.md) 决策日志（D-006 ～ D-009 状态改为已定）；
  - Owner：Product Owner × AI（进行中，见决策日志）。

- [ ] **T-002 仓库工程基线**
  - 产出：目录骨架（按 TECH_SPEC §22：apps / services / contracts / database / standards / evals / infra / docs）、分支策略（main + 短生命周期分支）、PR 检查单（docs/03 §7）；
  - DoD：新成员 clone 后 5 分钟内知道「代码往哪儿放、提交怎么写」；Conventional Commits 生效（本仓库前几次提交即为示范）；
  - Owner：Application Lead。

- [ ] **T-003 本地开发环境**
  - 产出：`docker-compose.yml`（PostgreSQL 18 + MinIO）、`services/api`（Go 空壳）、`services/ai`（FastAPI 空壳）、`apps/web`（Next.js 空壳）、`Makefile`（dev / lint / test / migrate）；
  - DoD：`make dev` 一条命令起全栈空壳并互相探活；README 更新到「新成员 20 分钟可跑通」（TECH_SPEC §23）；
  - Owner：Application Lead + DevOps。

## B. 产品与标准（关键路径，最容易被低估）

- [ ] **T-004 IRRS v0.1 规则集编写**
  - 说明：规则集是本项目**最核心资产**（PRD §17.1 / TECH_SPEC §38），目前只有维度框架（D1–D7）和权重假设，**没有任何一条具体规则**。没有它，Slice 1 的 AI 评估无标可依；
  - 产出：`standards/irrs/0.1.0/rules.yaml`，首批 **30–50 条规则**，覆盖 D1–D7，重点先覆盖餐饮业态；每条含 code / title / dimension / description / severity_default / weight / evaluation_type / evidence_requirements / rationale / examples（格式见 docs/03 §6.6）；
  - DoD：拿一套真实门店材料（菜单 + 门头 + 扫码页）人工走查一遍，能用这批规则完整跑出一次 Audit 结论；severity 与 weight 由人审定；
  - Owner：Product Owner（AI 起草，人工审定）。

- [ ] **T-005 用户验证启动（与开发并行）**
  - 说明：PRD §21 的访谈计划目前一条都没执行，E2 证据为 0；验证结论会直接反馈进规则 severity 排序和 Demo 故事线；
  - 产出：3–5 个访谈（建议 1 餐饮 / 1 咖啡零售 / 1 文展空间），按 PRD §21.2 的 10 个问题提问，记录进 PRD Evidence Log；
  - DoD：每个访谈记录齐「近期事件 + 失败环节 + 已发生代价」三要素；对照 PRD §22 Kill Criteria 给出 Go / Reframe / Stop 建议；
  - Owner：Product Owner。

- [ ] **T-006 Golden Dataset v0 素材收集**
  - 说明：Eval 需要 20 个 case（PRD §16.1：8 FAIL / 5 WARN / 5 PASS / 2 证据不足）。**素材必须真实**（自己拍的门店材料），不要网图——评审看得出来，且有版权问题；
  - 产出：素材库 `evals/datasets/irrs-v0.1/`，每个 case 含预期 rule_id / status / severity 范围 / 必须引用的证据（格式见 TECH_SPEC §18）；
  - DoD：20 个 case 格式合法且与 T-004 的 rule code 对得上；
  - Owner：Product Owner + AI。

## C. MVP Slice 1 竖切（最先打通的一条线）

- [ ] **T-007 契约先行：API + Schema 骨架**
  - 产出：`contracts/openapi/` 最小 OpenAPI 3.1（project / evidence / audit / findings 四组端点，TECH_SPEC §9）；`contracts/json-schema/assessment.schema.json`；
  - DoD：Go 与 Python 两端都能从契约生成或校验类型；契约 lint 进 CI（哪怕 CI 只有这一步）；
  - Owner：AI Tech Lead + Application Lead。

- [ ] **T-008 模型 Provider Adapter + Prompt v1**
  - 产出：`StructuredModel` Protocol + 首个供应商 adapter（含 retry / fallback）+ `ai/prompts/extraction/v1.md`、`ai/prompts/assessment/v1.md`（版本化文件，不写死在代码里）；
  - DoD：业务层无供应商 SDK import；「一张菜单图 → 合法 assessment JSON」跑通；schema 校验失败走安全失败路径（TECH_SPEC §28）；请求元数据（prompt_version / token / cost）有记录；
  - Owner：AI Tech Lead。**依赖：T-001 的供应商决策（D-007）。**

- [ ] **T-009 Slice 1 打通（本机）**
  - 流程：Create Project → Upload Image（Signed URL → MinIO）→ AI Structured Assessment → Finding（含 evidence_refs）→ 前端 Evidence 展示；
  - DoD：PRD §12.4 的 Magic Moment 在本机可复现——**点开一个 Finding 能看到它引用的证据**；job 失败时用户看到明确的失败态而不是白屏（ADVICE G-10）；
  - Owner：全体。

---

## 阻塞与依赖

| 任务 | 阻塞点 | 需要谁解除 |
|---|---|---|
| T-001 | 时间线 / 供应商 / 团队 / 验证顺序 四项决策讨论中 | Product Owner |
| T-008 | 依赖 D-007（模型供应商） | Product Owner |

## Archive

- [x] **T-000 文档基线**：PRD v0.1 / TECH SPEC v0.1 入库；TODO / TODO_NEXT / COMMUNICATING / ADVICE / 工程规范 五份文档建立；仓库初始化并分步提交（2026-09-07）
