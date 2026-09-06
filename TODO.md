# TODO ｜ 当前周期任务清单

> **2026-09-07 审查补充**：已产出 [详细执行方案](docs/05_EXECUTION_PLAN_v0.1.md) 和 [审查记录](docs/04_DOCUMENT_REVIEW_2026-09-07.md)。「不足一周砍 Slice 3」等范围冲突已在 B01 处理：裁决以[需求矩阵](docs/requirements-matrix.md)与决策日志 D-011/D-012 为准（草案待 PO 确认）。T-001 仍未完成：D-006～D-009 尚未获 PO 确认；规则、代码和环境任务均未执行。

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

周期时长：按暂定时间线 D-006（2 周）校准为 5–7 个工作日；赛事时间确认后立即复校。时间不足时按 D-011 草案执行：先砍未拉入的 P1 与包装，必要时整体降级为「技术验证切片」并显式声明——不以 Slice 1–2 冒充完整 MVP，复测闭环属 P0-7（R-01）。

---

## A. 决策与基线

- [ ] **T-001 执行方案定稿**
  - 当前进展：执行方案草案已完成（B01–B16）；B01 已建立[需求矩阵](docs/requirements-matrix.md)与决策 D-010～D-014。确认时按 D-014 在决策日志**追加新行 supersedes 旧 ID**，不直接改写历史记录。
  - 产出：按确定的「时间线 / 模型供应商 / 团队配置 / 验证顺序」四项决策，更新本文件与 TODO_NEXT.md 的排期；同时审定范围草案 D-011 与语言策略 D-012；
  - DoD：四项决策的确认记录以 supersedes 行写入 [COMMUNICATING.md](COMMUNICATING.md) 决策日志；D-011/D-012 状态更新；[需求矩阵](docs/requirements-matrix.md)同步；
  - Owner：Product Owner × AI（进行中，见决策日志）。

- [x] **T-002 仓库工程基线**
  - 产出：目录骨架（TECH_SPEC §22：apps / services / contracts / database 待 B05 / standards / evals / infra 待部署期 / docs + scripts）、分支策略（main + `codex/<topic>` 短分支，一批一 PR）、PR 模板（.github/pull_request_template.md = docs/03 §7.2 检查单）、根 [AGENTS.md](AGENTS.md)；
  - DoD：新成员读 AGENTS.md + README 即知「代码往哪儿放、提交怎么写」；Conventional Commits 全部历史提交即为示范；
  - Owner：Application Lead。（2026-09-07 B04 完成）

- [~] **T-003 本地开发环境**
  - 当前进展（B04）：compose（postgres:18 + MinIO）、三端空壳（Go chi :8080 / FastAPI :8100 / Next.js 16.3.3 :3000）、liveness/readiness（缺配置返回可诊断 503）、Makefile + `scripts/dev.ps1` 等价入口、`.env.example`（无密钥）、离线 CI（.github/workflows/ci.yml）。三端 lint/type/test/build 与契约校验已在本机实际跑通。
  - **受阻**：本机 Docker Desktop 引擎未能就绪（`docker info` 持续 500，进程在但引擎管道无响应）——`compose up` 全栈探活验证待引擎恢复后补做；解除人：Product Owner（重启 Docker Desktop / 检查 WSL）。完成后 `make dev` 语义（infra+三端探活）达成并补记。
  - Owner：Application Lead + DevOps。

## B. 产品与标准（关键路径，最容易被低估）

- [ ] **T-004 IRRS v0.1 规则集编写**
  - 当前进展（B02）：首批 10 条草稿已入库（[standards/irrs/0.1.0/rules.yaml](standards/irrs/0.1.0/rules.yaml)，覆盖 D1–D7，餐饮优先，全部 `draft`）；规则 Schema、评分语义草案（[ADR-0002](docs/adr/0002-scoring-and-review.md)）与正反例 fixture 已建立；schema 校验脚本可离线运行。**未完成**：PO 逐条审定 severity/weight、扩充到 30–50 条、真实材料人工走查——完成前规则包保持 draft，不得用于生产评分。
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
  - 当前进展（B03）：`contracts/` 已建立——[OpenAPI 3.1](contracts/openapi/arrivalready.yaml)（TECH_SPEC §9 全量 13 条端点 + 幂等键 + RFC 9457 错误）、四份 JSON Schema（normalized_evidence / assessment / provider_response / job_payload）、15 个共享正反例 fixture、[状态迁移表](contracts/state-machines.md)（三条状态轴分离，关闭 R-05/R-06 契约面）；离线校验脚本全绿。**未完成**：Go/Python 双端从契约生成或校验类型、spectral lint 进 CI（随 B04 落地）。
  - 产出：`contracts/openapi/` 最小 OpenAPI 3.1（project / evidence / audit / findings 四组端点，TECH_SPEC §9）；`contracts/json-schema/assessment.schema.json`；
  - DoD：Go 与 Python 两端都能从契约生成或校验类型；契约 lint 进 CI（哪怕 CI 只有这一步）；
  - Owner：AI Tech Lead + Application Lead。

- [ ] **T-008 模型 Provider Adapter + Prompt v1**
  - 产出：`StructuredModel` Protocol + 首个供应商 adapter（含 retry / fallback）+ `ai/prompts/extraction/v1.md`、`ai/prompts/assessment/v1.md`（版本化文件，不写死在代码里）；
  - DoD：业务层无供应商 SDK import；「一张菜单图 → 合法 assessment JSON」跑通；schema 校验失败走安全失败路径（TECH_SPEC §28）；请求元数据（prompt_version / token / cost）有记录；
  - Owner：AI Tech Lead。**依赖：D-007 供应商已暂定（豆包主 + Qwen-VL 备，待 PO 复核）；若改选其他家，只影响 adapter，不影响本任务结构。**

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