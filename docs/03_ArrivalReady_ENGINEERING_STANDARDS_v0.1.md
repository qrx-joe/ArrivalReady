# Arrival Ready｜迎客验收 — 工程规范与制作标准（Engineering & Delivery Standards）

> 文档版本：v0.1
> 日期：2026-09-07
> **文档定位**：TECH_SPEC（docs/02）回答「**用什么造、架构长什么样**」；本文档回答「**怎么造、什么算造得好**」。两者冲突时以 TECH_SPEC 为准并在此登记。
> 适用范围：本仓库全部代码、Prompt、规则集、迁移脚本与文档。

---

# 1. 选型总原则

所有技术决策（含未来变更）必须能通过以下 6 条检验：

1. **成熟优先于新颖**：选「无聊的、被大规模验证过的」技术；追新只在有明确收益且可回退时进行；
2. **契约先行**：跨语言 / 跨服务边界先写契约（OpenAPI / JSON Schema），再写实现；
3. **模块化单体**：先一个能跑的整体，负载证明了再拆（TECH_SPEC §2.1）；
4. **确定性内核、概率性边缘**：权限、状态机、评分、历史必须确定性；AI 只在边缘（TECH_SPEC §2.3）；
5. **最小依赖面**：每引入一个依赖都要能回答「删掉它产品缺什么」；框架不得渗入 domain 层；
6. **可替换性**：模型、存储、身份 provider 都必须能在一个 adapter 内更换，业务层无感。

---

# 2. 技术选型（含依据）

## 2.1 选型总表

| 层 | 选型 | 版本基线 | 职责 |
|---|---|---|---|
| Web | Next.js（App Router）+ React + TypeScript（strict） | Next **16.3.x**（2026-08 安全更新版，修复两个 Critical RCE，**禁止低于此版本**）/ React 19.x | 用户体验层 |
| API | Go + `chi` + `pgx` + `sqlc` | Go **1.27.x**（2026-08-19 发布） | 确定性业务系统 |
| AI | Python + FastAPI + Pydantic v2 + httpx | Python **3.13.x**（3.14 待 AI SDK 兼容性验证后升） | 概率性 AI / Eval |
| DB | PostgreSQL（pgvector 按需启用） | 18.x | 唯一主数据存储 |
| 对象存储 | S3 兼容（生产：云存储；本地：MinIO） | — | Evidence 文件 |
| 观测 | OpenTelemetry + slog（Go）/ structlog（Py） | — | 可观测性 |
| 模型 | Provider Adapter（**1 主 + 1 备**） | 供应商待定（D-007） | 视觉 / 结构化 / 嵌入 |

> 版本核对记录：Next.js 16.3.3 与 Go 1.27 已于 2026-09-07 经官方渠道核实；Python / PostgreSQL / React 版本在工程基线日（T-003）以官方 release 页最终 pin。
> B04 复核（2026-09-07，工程基线日）：Next.js 16.3.3（官方 blog，Active LTS，修复 2 个 Critical）✅；Go 1.27.1（go.dev release notes，2026-09-01 发布）✅，本机工具链 1.26.2 经 go.mod toolchain 指令自动升级；React 19.2.8（npm registry 19.2 线最新）✅；Python 3.13.6 本机 + CI 3.13 ✅。PostgreSQL `postgres:18` 与 MinIO 镜像 tag/digest 在本机 Docker 引擎恢复后执行 `compose up` 时落 pin（当前 MinIO 为 dev-only 浮动 tag，禁止上生产）。

## 2.2 为什么这套选型贴合当前（2025–2026）行业主流

方向性说明（不做具体跑分引用）：

- **TypeScript 全栈 + 严格类型**仍是 Web 侧最大公约数；Next.js 已建立 LTS 节奏，跟随其安全更新线是成本最低的安全策略；
- **SQL-first 复兴**：`sqlc` 代表的「写 SQL、生成类型安全代码」路线在 Go 社区是主流收敛位，避免 ORM 魔法；
- **Python + FastAPI + Pydantic v2** 是 AI 服务的事实标准栈；Pydantic 同时承担「边界校验 + JSON Schema 生成」双重职责；
- **Structured Outputs（原生 JSON Schema）**已取代「prompt 里求模型输出 JSON」，成为所有主流供应商（含国产）的标准能力，是 Evidence-grounded 系统的可靠性基座；
- **Postgres 一库多用**（关系 + JSONB + full-text + pgvector）取代「为 RAG 单独上向量库」，是行业明确的合并趋势；
- **Modular Monolith 回潮**：业界已充分反思无差别微服务化，中小团队默认单体 + 清晰模块边界；
- **OpenTelemetry** 是厂商中立观测的既定标准，避免锁定 APM 供应商。

我们的选型不是标新立异，而是**站在每个位置的主流收敛位上**，用最小的依赖面把它们接起来。

## 2.3 明确不选的东西（及重启评估条件）

| 不选 | 原因 | 重启评估条件 |
|---|---|---|
| Kubernetes | 运维成本 >> 当前规模 | TECH_SPEC §25.3 K8s Gate（≥2 项） |
| Kafka / 独立 MQ | Postgres jobs table 足够 | job 积压 / 多分钟级 workflow（TECH_SPEC §10） |
| 独立向量数据库 | pgvector 覆盖当前规模 | 检索 QPS / 数据量成为瓶颈 |
| LangChain 作为业务骨架 | 框架锁定、难测、Prompt/Schema/Rule 关系不透明 | 允许局部组件使用，主干始终是普通 Python（TECH_SPEC §3.3） |
| 独立 Java 业务微服务 | 无对应负载；DevOps 能力投向交付与稳定性更有价值 | 出现真实 JVM 生态依赖（TECH_SPEC §33） |

---

# 3. 同类产品的制作规范参考

不重复发明轮子：以下产品在各自领域已被验证的制作规范，直接内化为我们的工程要求。

| 参照对象 | 它做对了什么 | 内化为我们的哪条规范 |
|---|---|---|
| **PersonaQA / Synthetic Users**（AI 体验测试） | Finding 必须携带证据与定位（截图 / DOM 区域）；运行可重复、可比较 | §5.3：AuditRun 不可变 + Diff；Finding 必须绑定 FindingEvidence；locator_json 全类型支持 |
| **Applause**（真人本地化测试） | 结构化用例分类学 + severity 分级 + 覆盖矩阵 | §5.2：IRRS 的「维度 × 旅程阶段 × 严重度」三维分类，规则必须归位到三维坐标 |
| **Lokalise / Phrase**（本地化平台） | 内容生命周期状态机（machine_draft → human_reviewed → published），状态可审计 | §5.4：一切状态机显式定义、非法迁移抛错、迁移写 Audit Log（翻译状态字段直接沿用此三态） |
| **SafetyCulture 等巡检验收产品** | 检查表版本化；整改 Action 带 owner / due / 复验闭环 | §5.5：StandardVersion 不可变 + FixTask（owner/due）+ Retest 闭环 |
| **Linear**（工程效率产品） | 克制的状态机、键盘优先、性能预算、opinionated workflow | §6 UI 规范：页面少而深；每个页面必须回答「用户在这里做什么决定」 |
| **Stripe / Vercel**（API 与文档工程） | 契约版本化、changelog 纪律、错误模型全局一致 | §4：API 规范；§8：版本与发布规范 |

**提炼出的一条元规范**：以上产品的共同点是「**结论可追溯、过程可复现、状态可审计**」。我们的每一层设计都要能回答这三个问题。

---

# 4. 行业规范遵循清单

每条规范都必须落到「我们怎么遵守」，不挂空名。

## 4.1 API 与网络

| 规范 | 我们怎么遵守 |
|---|---|
| OpenAPI 3.1 | `contracts/openapi/` 是 API 唯一事实源；CI 做 lint + 破坏性变更 diff |
| RFC 9457 Problem Details | 全部错误响应统一 `type/title/status/detail/instance/request_id`（TECH_SPEC §9.5） |
| Idempotency-Key（IETF draft） | 5 个写操作必须支持幂等：create audit / upload complete / retest / publish / webhook（TECH_SPEC §11） |
| RFC 3339 / UTC | 时间一律 UTC 存储、ISO8601 传输、客户端本地化展示 |
| BCP 47 | locale 字段一律 `en-US` 式标签；语言 ≠ 地区，不合并字段 |
| Cursor 分页 | 列表端点统一 cursor 分页，不用 offset 裸翻页 |

## 4.2 版本与标识

| 规范 | 我们怎么遵守 |
|---|---|
| SemVer 2.0.0 | IRRS 标准、API 契约、镜像 tag 三处严格三段式版本 |
| UUIDv7 | 全部主键（时间有序，利于索引与调试） |
| Keep a Changelog | 首次对外发布时建立 `CHANGELOG.md`，此前用 git tag 即可 |

## 4.3 安全与合规

| 规范 | 我们怎么遵守 |
|---|---|
| OWASP API Security Top 10 | BOLA 必须有自动化测试（E2E 路径 3，TECH_SPEC §19.4）；authz 与业务逻辑分离为独立中间件 |
| OWASP ASVS | MVP 按其清单的认证 / 会话 / 文件上传章节自查；试点期不追求全级别认证 |
| WCAG 2.2 AA | 公众 Guest Page 强制；运营端至少满足 keyboard / focus / contrast 基本项 |
| PIPL / 数据最小化 | TECH_SPEC §15 落地为代码检查项：新收集字段必须在 PR 里回答「删掉它验收流程还成立吗」 |
| 《生成式 AI 服务管理暂行办法》精神 | 面向公众的 AI 生成内容（Guest Layer）必须经人工 review 后发布（publish gate），并有可识别的生成标记；MVP 内部试用阶段同样保留 review 位 |
| 密钥管理 | `.env` 不入库（.gitignore 已覆盖）；生产 secret 走部署环境注入；代码出现硬编码 secret = 最高优先级修复 |

## 4.4 工程过程

| 规范 | 我们怎么遵守 |
|---|---|
| Conventional Commits 1.0.0 | 全仓库强制；type 限定：feat / fix / docs / chore / refactor / test / build / ci |
| Trunk-Based Development | main 受保护；feature 分支存活 ≤ 2 天；squash merge |
| MADR | `docs/adr/NNNN-标题.md`；触发清单见 TECH_SPEC §21.5 |
| C4 Model | 架构图用 C4 的 Context / Container 两级就够，不画装饰图 |

---

# 5. 可扩展性与可维护性设计规范

## 5.1 依赖方向（强制）

```text
apps/web ──────→ contracts ←────── services/api
                      ↑                  │
                      └──────────────────┤
                                         ↓ (HTTP + JSON Schema)
                                   services/ai
```

- `contracts/` 是唯一共享物；三个应用之间**不共享代码**，只共享契约；
- 每个 service 内部三层，依赖单向：`transport（协议层）→ domain（业务层）← infra（外部世界）`；
- **domain 层禁止 import**：chi、fastapi、数据库驱动、供应商 SDK。违反 = PR 直接打回。

## 5.2 扩展点清单（扩展只动这些地方）

| 要扩展什么 | 只允许动哪里 | 检验标准 |
|---|---|---|
| 加一条 IRRS 规则 | `standards/irrs/x.y.z/rules.yaml` 加一条 + 一个 eval case | **零应用代码改动** |
| 换 / 加模型供应商 | `services/ai/providers/` 加一个 adapter | 业务层 diff 为 0 |
| 加一种 Evidence 类型 | evidence handler 注册表加一个实现 | pipeline 主干不改 |
| 换对象存储 | `Storage` 接口另一实现 | 同上 |
| 换身份 provider | authn adapter | 同上 |
| 改评分模型 | 新 StandardVersion（数据），不是改代码 | 历史 AuditRun 不可变 |

**扩展性验收问句**：任何「加 X」的需求，如果答案不是「加数据 / 加一个实现类」，而是「改主干」，先停下来写 ADR。

## 5.3 不可变与可追溯（产品承诺的工程化）

- AuditRun 一旦完成即不可变；Retest = 新 run（TECH_SPEC §6.1）；
- StandardVersion 发布后不可改；改动 = 新版本；
- 一切状态迁移写 Audit Log（actor / before / after / request_id）；
- 删除只有软删除；物理删除只走 retention 流程（TECH_SPEC §15.2）。

## 5.4 契约演进

- API 只加不改（additive-first）；破坏性变更 = `/api/v2` 或字段废弃窗口（标记 deprecated ≥ 1 个版本）；
- JSON Schema 变更必须向后兼容或同步升 schema version，并在 evals 里补 case；
- 数据库迁移规则沿用 TECH_SPEC §31（全部走 migration、向前兼容、破坏性迁移要备份 + ADR）。

---

# 6. 注释与文档规范（详细）

> 用户明确要求「写注释（详细）」。但**详细 ≠ 每行翻译代码**：无信息量的注释是负资产。
> 我们的注释标准是：**给 6 个月后的同事看，他能不问你就安全地改这段代码。**

## 6.1 三条哲学

1. **解释为什么，不解释是什么**：代码已经说了是什么；注释的价值是代码说不出的——约束、取舍、坑、来源；
2. **未来读者原则**：读者不是编译器，是赶工期时来改 bug 的下一个人（可能就是你自己）；
3. **过时的注释比没有更糟**：改行为必须同步改注释；PR 评审项之一就是「注释还成立吗」。

## 6.2 必须写注释的 9 类位置

1. **安全边界**：SSRF 校验、上传校验、authz 判断——写明「防什么攻击、为什么放这里」；
2. **业务规则与魔法数字**：权重、限流阈值、保留天数——写明出处与修改代价；
3. **状态机约束**：为什么某条迁移被禁止；
4. **外部系统 / 模型的坑**：供应商的怪癖、超时行为、配额限制；
5. **AI Prompt 的业务约束**：业务限制为什么这样表述、禁止模型做什么、对应哪个 eval case；
6. **非直观算法**：评分、diff、去重的正确性论证；
7. **Workaround**：修了什么上游问题，附 issue / 文档链接，写明何时可以删；
8. **不可逆操作**：迁移、物理删除、发布 gate；
9. **不做的决定**：为什么这里**没有**做某事（防止后人好心补上，踩回我们绕开的坑）。

## 6.3 Go 注释规范

- 所有导出符号必须有 godoc 注释，以符号名开头；
- package 注释写**职责与不变量**，不写实现清单；
- 未导出但反直觉的逻辑用行注释解释 why。

```go
// Package scoring implements deterministic Readiness Score calculation.
//
// Invariants:
//   - Score is computed ONLY from findings bound to the StandardVersion
//     the audit run was executed against — never from the currently
//     active version. Historical runs must stay reproducible because
//     before/after comparison is a core product promise (PRD §12.3).
//   - LLM output never feeds this package directly; it must pass
//     grounding validation and human review first (TECH_SPEC §2.3).
package scoring

// Compute aggregates dimension scores for a completed audit run.
//
// Findings with status UNKNOWN are excluded from BOTH numerator and
// denominator, and the result reports coverage so the UI can render
// "scored on N of M rules". This is deliberate: penalizing UNKNOWN
// would incentivize the AI stage to avoid returning UNKNOWN, which we
// never want — a forced answer is worse than an honest unknown
// (TECH_SPEC §28 "Evidence insufficient").
func Compute(findings []Finding, weights RuleWeights) (Score, error)
```

TODO 格式（必须可执行，不许许愿）：

```go
// TODO(qiaoruixue, TD-003): switch to provider-native structured mode once
// SDK v2 ships; until then keep the repair-retry path. Context: SDK issue #142.
```

## 6.4 Python 注释规范

- module docstring 写**契约**：输入 / 输出 / 约束（Google style）；
- **Pydantic 的 `Field(description=...)` 是注释体系的一部分**：它会进入 JSON Schema，被发给模型、出现在 API 文档里——按「对外契约」的标准维护，不是普通注释；
- 行注释解释 why，保持 TECH_SPEC §21.4 的风格。

```python
"""Stage 4 — Rule assessment.

Contract:
    Input:  NormalizedEvidence + Rule candidates selected in Stage 3.
    Output: list[Assessment] strictly validating assessment.schema.json.

Constraints:
    - The model may return UNKNOWN; it must never be pushed into a
      PASS/FAIL answer when evidence is insufficient (PRD §9 "No Fake
      Certainty").
    - Every FAIL/WARN must reference an evidence id present in the
      input set. Stage 5 re-validates references, but obvious
      fabrications are dropped here to fail fast and cheap.
"""

class Assessment(BaseModel):
    rule_id: str = Field(
        description="IRRS rule code (e.g. IRRS-D4-001). Must exist in the "
        "rule set bound to this audit run; unknown codes are rejected.",
    )
    confidence: float = Field(
        ge=0,
        le=1,
        description="Model self-reported confidence. This is a triage hint, "
        "NOT a correctness probability; reviewers see it but scoring "
        "ignores it.",
    )
```

## 6.5 TypeScript / React 注释规范

- 域类型、hooks、复杂组件写 JSDoc：职责 + 关键约束；
- 组件注释必须包含**无障碍契约**（面向 WCAG 2.2 AA 的部分）；
- props 简单且自解释的不逐项注释，避免噪音。

```tsx
/**
 * EvidenceViewer renders one evidence item with optional locators
 * (bounding box / PDF page / DOM region). It is the core of the
 * "Evidence First" promise (PRD §9): every finding must be human
 * verifiable in one click. Do not add features that draw attention
 * away from the locator overlay.
 *
 * Accessibility: locator hotspots are keyboard-focusable and expose
 * aria-labels built from locator metadata (WCAG 2.2 AA target,
 * TECH_SPEC §16.1).
 */
export function EvidenceViewer({ evidence, locators }: Props) { ... }
```

## 6.6 YAML（IRRS 规则）注释规范

每条规则**必须**包含 description / rationale / evidence_requirements / examples。`rationale` 是人工定 severity 与 weight 的依据，也是未来评审质疑时的答辩词。

```yaml
- code: IRRS-D4-001
  title: QR 点单流程语言可达性
  dimension: D4                      # 旅程阶段 Act（PRD §5.1）
  description: >
    扫码点单落地页必须让目标访客语言下可独立完成：选品、数量、备注、下单确认。
  severity_default: S1
  weight: 3
  evaluation_type: ai
  evidence_requirements:             # 没有这两类证据时必须返回 UNKNOWN，禁止凭菜单推断
    min_count: 1
    must_show: [qr_landing_page]
  rationale: >
    点单是餐饮业态核心转化动作；语言不可达约等于订单损失。S1 而非 S0：
    存在店员协助的降级路径（访谈证据 2026-09-xx 后复核）。
  examples:
    - fail: 落地页仅中文且无语言切换
    - pass: 提供 en/ja 切换且价格品类完整可见
```

## 6.7 SQL 迁移注释规范

每个迁移文件头部写三行：**意图 / 回滚策略 / 是否破坏性**。

```sql
-- 20260907_0001_create_audit_run.sql
-- Intent: create audit_run per TECH_SPEC §5.7 (immutable runs, retest = new row).
-- Rollback: DROP TABLE audit_run (safe only before findings FK exists;
-- after that this is destructive → backup + ADR required).
-- Destructive: no
```

## 6.8 Prompt 文件注释规范

每个 Prompt 文件（`ai/prompts/**/*.md`）头部写 front-matter 式说明：version / goal / constraints / prohibited_behaviors / 对应 eval case 列表。**改 Prompt 必须跑 eval**（TECH_SPEC §8.2），版本号 +1。

## 6.9 禁止的注释

```go
// userID 是用户 ID          ← 翻译代码，禁止
// handle error              ← 无信息量，禁止
// temp fix                  ← 无 context、无 owner、无期限，禁止
// 这里本来想用 X，但是……（无链接无结论） ← 要么写成完整 why，要么删掉
```

- 注释掉的死代码：直接删，git 会记得；
- 情绪化 / 指向具体他人的注释：禁止。

## 6.10 文档要求（除注释外）

| 文档 | 要求 | 建立时机 |
|---|---|---|
| 每个 service 的 README | 怎么跑、怎么测、边界在哪 | T-003 |
| `docs/adr/` | MADR 格式，索引表在目录 README | 首个 ADR 触发时 |
| 术语表 | 见本文档 §9 | 已建立，随项目维护 |
| Runbook | 部署步骤、常见故障 | 首次部署前 |

---

# 7. 质量门禁

## 7.1 静态检查矩阵（CI 必须包含）

| 层 | Format | Lint | 类型 | 测试 |
|---|---|---|---|---|
| apps/web | Biome 或 Prettier（二选一锁定） | ESLint 或 Biome | `tsc --noEmit`（strict） | Vitest + Playwright（关键路径） |
| services/api | gofmt | golangci-lint | go vet | go test + ephemeral PostgreSQL |
| services/ai | ruff format | ruff check | mypy 或 pyright | pytest |
| contracts | — | spectral（OpenAPI lint） | — | JSON Schema fixture 校验 |

## 7.2 PR 检查单（合并前自查）

- [ ] 提交信息符合 Conventional Commits
- [ ] 新行为有测试，**至少覆盖失败路径**
- [ ] lint / format / type 全绿
- [ ] 错误 / 加载 / 空状态已处理（ADVICE G-10）
- [ ] 注释符合 §6：新增魔法数字有 why；改行为的注释已同步
- [ ] 涉及契约：OpenAPI / JSON Schema 已同步并过 diff 检查
- [ ] 涉及 AI：相关 eval case 已更新
- [ ] 涉及标准：版本号已处理（禁止原地改已发布版本）
- [ ] 无硬编码 secret；新字段能回答「数据最小化」问句（§4.3）

## 7.3 Definition of Done

沿用 TECH_SPEC §34，不重复。

---

# 8. 版本、分支与发布规范

- **分支**：main 受保护；分支名 `feat/xxx`、`fix/xxx`、`docs/xxx`；存活 ≤ 2 天；
- **合并**：squash merge；小团队阶段允许自审自并，但 §7.2 检查单必须逐项过；
- **批次边界**（2026-09-07 B01 补充，D-013）：一个执行批次一个短分支与一个 PR；squash 只限本批，禁止把多个执行批次合成一个提交/PR（执行方案 §7.1）；
- **提交**：Conventional Commits；一步一提交（一个完整最小变更 = 一次提交）；
- **里程碑**：比赛 / 演示节点打 tag（如 `v0.1.0-demo`），tag 必须指向 CI 全绿的 commit；
- **镜像**：禁止 `latest` 上生产；tag 与 git commit 关联（TECH_SPEC §24）；
- **CHANGELOG**：首次对外发布时按 Keep a Changelog 建立。

---

# 9. 术语表

| 术语 | 含义 |
|---|---|
| Evidence | 支撑验收结论的原始材料（图片 / PDF / URL / 文本） |
| Finding | 针对某条规则的一次结论（PASS/WARN/FAIL/UNKNOWN），必须引用 Evidence |
| AuditRun | 一次不可变的验收运行；Retest 产生新 run |
| IRRS | Internal Readiness Standard，内部接待准备度标准（版本化） |
| Journey Stage | 国际访客旅程阶段：Discover/Understand/Decide/Act/Pay/Recover/Remember |
| Grounding | Finding 与 Evidence 之间的可验证绑定关系 |
| Golden Dataset | 带预期输出的固定测试集，用于 AI 回归评估 |
| Guest Layer | 面向国际访客的过渡性补充页面（整改手段之一，非产品本体） |
| S0–S3 / Info | 严重度分级：Blocker / Critical / Major / Minor / 建议项 |
| E0–E4 | 机会证据阶梯（option-skill）：假设 → 信号 → 事件+代价 → 行为 → 付费 |

---

# 10. 参考清单（检索 / 核对日期：2026-09-07）

1. Next.js August 2026 Security Release（16.3.3）：https://nextjs.org/blog/august-2026-security-release
2. Go 1.27 Release Notes：https://go.dev/doc/go1.27
3. OWASP API Security Top 10 (2023)：https://owasp.org/API-Security/
4. OWASP ASVS：https://owasp.org/www-project-application-security-verification-standard/
5. WCAG 2.2：https://www.w3.org/TR/WCAG22/
6. OpenAPI 3.1：https://spec.openapis.org/oas/v3.1.0
7. RFC 9457 Problem Details：https://www.rfc-editor.org/rfc/rfc9457
8. SemVer 2.0.0：https://semver.org/
9. Conventional Commits 1.0.0：https://www.conventionalcommits.org/
10. Keep a Changelog：https://keepachangelog.com/
11. MADR（ADR）：https://adr.github.io/madr/
12. C4 Model：https://c4model.com/
13. OpenTelemetry：https://opentelemetry.io/docs/
14. sqlc：https://docs.sqlc.dev/
15. Pydantic v2：https://docs.pydantic.dev/
