# Arrival Ready｜迎客验收 — 技术规范文档（Technical Specification）

> 文档版本：v0.1  
> 日期：2026-09-07  
> 同步注记（2026-09-07，B01）：§8.1 目录来源已按 [ADR-0001](adr/0001-canonical-repository-layout.md) 修订（关闭审查项 R-09）；范围映射见[需求矩阵](requirements-matrix.md)。其余章节不变。  
> 目标：支持 Competition MVP，同时保证后续可扩展、可维护、可测试、可观测、可替换模型  
> 原则：**先模块化单体 / 少服务，后按真实负载拆分；不为黑客松制造分布式系统。**

---

# 1. 技术目标

Arrival Ready 的技术系统必须支持：

1. 多来源 Evidence（图片、PDF、URL、文本）；
2. AI 结构化理解；
3. 版本化 Readiness Standard / Rule；
4. Evidence-grounded Finding；
5. 确定性评分；
6. 人工 Review；
7. Fix Workflow；
8. Retest / Regression / History；
9. AI Eval；
10. 可观测、可审计、可替换模型。

系统必须避免：

- 核心业务由 Prompt 字符串隐式控制；
- LLM 直接自由生成总分；
- 前端解析 LLM Markdown；
- 把模型供应商类型传播到整个业务层；
- 为了“技术栈齐全”同时创建无必要的 Go / Java / Python 微服务；
- 没有 Evidence 的 Finding 被当作事实。

---

# 2. 架构原则

## 2.1 Modular Monolith First

MVP 采用：

- 一个 Web App；
- 一个 Go Application API；
- 一个 Python AI Worker/Service；
- PostgreSQL；
- S3-compatible Object Storage；
- 可选 Redis。

不引入：

- Kafka；
- Kubernetes；
- Service Mesh；
- 独立 Java 业务微服务；
- 复杂 Event Sourcing；
- 多数据库分片。

这些不是“更生产”，只是会让两天 MVP 更擅长生产 YAML。

## 2.2 Contract First

跨语言边界必须有明确契约：

- HTTP API：OpenAPI 3.1；
- AI Output：JSON Schema；
- Event / Job Payload：JSON Schema；
- 数据库：Migration 管理；
- 业务常量：版本化。

## 2.3 Deterministic Core, Probabilistic Edge

概率性 AI 放在边缘：

```text
Evidence
  ↓
AI Extraction / Assessment
  ↓
Structured Candidate Findings
  ↓
Rule Validation
  ↓
Human Review（必要时）
  ↓
Deterministic Scoring
  ↓
Report
```

系统中以下必须确定性：

- 权限；
- 状态机；
- 总分计算；
- 历史版本；
- Audit Diff；
- Publish Gate；
- 高风险规则；
- 数据保留策略。

---

# 3. 推荐技术栈

## 3.1 Frontend

### 默认

- **Next.js 16.3.3 Active LTS**
- React 19.2
- TypeScript（strict）
- pnpm
- CSS Modules + CSS Custom Properties / Design Tokens
- Radix UI primitives（仅在无障碍交互复杂时使用）
- React Hook Form + Zod
- TanStack Query（仅客户端远程状态需要时）
- Playwright E2E

### 为什么

Next.js 16 当前稳定分支已经围绕 App Router、Server Components、Turbopack 和现代导航能力成熟；2026-08 的安全公告要求更新到 16.3.3，因此明确 pin 到安全修复版本，而不是模糊写 `latest`。

### UI 规范

- 不允许业务组件散落大量硬编码颜色/间距；
- Design Token 用 CSS Variables；
- 组件按 `primitive → domain → page` 三层组织；
- 运营端 desktop-first + responsive；
- 访客 Guest Page mobile-first；
- 对公众页面以 WCAG 2.2 AA 为最低目标。

---

## 3.2 Application Backend

### 默认

- **Go 1.27.1**
- `net/http` + `go-chi/chi`（轻量、贴近标准库）
- `pgx` + `sqlc`
- PostgreSQL migrations：`golang-migrate` 或 Atlas（二选一，项目启动后锁定）
- OpenAPI 3.1
- `oapi-codegen`（可选）
- `slog` JSON structured logging
- OpenTelemetry Go SDK

### 为什么 Go

- 团队已有 Go 全栈成员；
- API、文件元数据、状态机、并发任务协调适合 Go；
- Go 兼容性承诺有利于长期维护；
- 避免在核心业务层引入过重 ORM 魔法。

### 后端职责

- AuthN/AuthZ；
- Project；
- Evidence metadata；
- Audit Run；
- Finding；
- Review；
- Fix Task；
- Scoring；
- Retest；
- Guest Layer publish；
- AI Job orchestration；
- Rate limit；
- Audit log。

---

## 3.3 AI Service

### 默认

- Python **3.13.x** 作为第一阶段生产兼容基线；
- 兼容性验证后升级 Python 3.14.x；
- FastAPI；
- Pydantic v2；
- httpx；
- pytest；
- Ruff；
- mypy/pyright；
- OpenTelemetry Python SDK。

> 2026-08 Python 3.14.7 已发布，但 AI SDK/解析库兼容性应先验证。规范选择“成熟生态兼容性”而不是为了版本号好看追最新。

### AI 编排原则

首版**不依赖 LangChain/LlamaIndex 作为业务骨架**。

允许它们用于局部组件，但核心 Pipeline 明确写成普通 Python domain services：

- ingest；
- extract；
- classify；
- retrieve；
- assess；
- validate；
- suggest_fix；
- localize。

理由：

- 降低框架锁定；
- 更容易做 unit test / eval；
- Prompt、Schema、Rule 关系更透明；
- 以后可替换为其他 Agent framework。

---

## 3.4 Model Provider

实现 `ModelProvider` 接口：

```text
VisionModel
StructuredGenerationModel
EmbeddingModel
```

首版只启用 1 个主供应商 + 1 个 fallback，不同时接 5 家模型制造“多模型架构”。

### Provider Adapter

```python
class StructuredModel(Protocol):
    async def generate(self, request: ModelRequest, schema: type[T]) -> T:
        ...
```

业务层只能依赖 `StructuredModel`，不得直接 import 某供应商 SDK。

### Structured Output

优先使用供应商原生 JSON Schema / Structured Output；仍需 Pydantic 二次校验。

**结构正确 ≠ 内容正确**，所以 schema valid 只是第一层 gate，Evidence Grounding 和 Eval 仍必须存在。

---

# 4. 数据层

## 4.1 PostgreSQL

推荐：**PostgreSQL 18.x**。

原因：

- 关系型业务数据强；
- ACID；
- JSONB；
- Full-text；
- UUIDv7；
- 可通过 pgvector 添加向量检索；
- 不需要为 RAG 再引入独立向量数据库。

## 4.2 pgvector

仅当出现真实 Retrieval 需求时启用。

用途：

- IRRS rule semantic retrieval；
- Fix Library；
- 历史相似 Finding；
- 商户资料片段检索。

不要把所有文本自动 embedding。当数据规模很小，SQL / keyword filter 比向量检索更可解释。

## 4.3 Object Storage

S3-compatible：

- Production：云对象存储；
- Local：MinIO；
- 通过统一 Storage interface。

禁止：

- 把用户文件直接存数据库 BYTEA；
- 永久公开 URL；
- 前端持有云 Secret。

采用 Signed URL。

---

# 5. 核心领域模型

## 5.1 Organization

```text
Organization
- id UUIDv7
- name
- plan
- created_at
```

## 5.2 User

```text
User
- id
- organization_id
- role
- email
- status
```

Roles：

- OWNER
- ADMIN
- REVIEWER
- MEMBER
- VIEWER

MVP 可简化，但 schema 预留 organization。

## 5.3 Project

```text
Project
- id
- organization_id
- name
- entity_type (restaurant / retail / museum / venue / other)
- target_locale
- target_profile_id
- status
- created_by
- created_at
```

## 5.4 Evidence

```text
Evidence
- id
- project_id
- type (image/pdf/url/text/video)
- source_uri
- object_key
- sha256
- mime_type
- journey_stage
- captured_at
- uploaded_by
- processing_status
- extracted_text
- metadata_json
- pii_redaction_status
```

## 5.5 StandardVersion

```text
StandardVersion
- id
- code = IRRS
- version = 0.1.0
- status (draft/active/retired)
- published_at
```

## 5.6 Rule

```text
Rule
- id
- standard_version_id
- code
- dimension
- title
- description
- severity_default
- weight
- evaluation_type (deterministic / ai / hybrid / human)
- evidence_requirements_json
- active
```

## 5.7 AuditRun

```text
AuditRun
- id
- project_id
- standard_version_id
- parent_run_id nullable
- model_bundle_version
- prompt_bundle_version
- started_at
- completed_at
- status
- total_score
```

## 5.8 Finding

```text
Finding
- id
- audit_run_id
- rule_id
- issue_type
- journey_stage
- status (PASS/WARN/FAIL/UNKNOWN)
- severity
- title
- description
- confidence
- recommended_fix
- ai_generated boolean
- review_status
- created_at
```

## 5.9 FindingEvidence

```text
FindingEvidence
- finding_id
- evidence_id
- locator_json
- quote_or_observation
```

`locator_json` 支持：

- 图片 bounding box；
- PDF page；
- URL DOM selector / screenshot region；
- Video timestamp。

## 5.10 Review

```text
Review
- id
- finding_id
- reviewer_id
- decision (confirm/reject/edit/na)
- note
- created_at
```

## 5.11 FixTask

```text
FixTask
- id
- finding_id
- owner_id
- status
- due_at nullable
- resolution_note
```

---

# 6. 状态机

## 6.1 AuditRun

```text
DRAFT
 → QUEUED
 → INGESTING
 → ANALYZING
 → REVIEW_REQUIRED
 → COMPLETED

任意阶段 → FAILED / CANCELLED
```

禁止直接从 `DRAFT → COMPLETED`。

## 6.2 Finding

```text
OPEN
 → ACKNOWLEDGED
 → FIXING
 → READY_FOR_RETEST
 → RESOLVED

OPEN/FIXING → ACCEPTED_RISK
READY_FOR_RETEST → REOPENED（复测失败）
```

状态改变写入 Audit Log。

---

# 7. AI Pipeline

## 7.1 Stage 1 — Ingest

输入：Evidence。

操作：

- MIME validation；
- hash；
- metadata；
- file scanning；
- optional PII redaction；
- OCR / document extraction。

输出：`NormalizedEvidence`。

## 7.2 Stage 2 — Scene Extraction

从 Evidence 中结构化提取：

- observable text；
- UI controls；
- price；
- payment markers；
- language coverage；
- service instructions；
- cultural terms；
- accessibility cues；
- uncertain observations。

必须区分：

- `observed_fact`；
- `inference`；
- `unknown`。

## 7.3 Stage 3 — Rule Candidate Selection

先用 deterministic filter：

- entity_type；
- journey stage；
- evidence type；
- locale。

数据多后才用 pgvector 辅助找候选 Rule。

## 7.4 Stage 4 — Assessment

每个 Rule 输出：

```json
{
  "rule_id": "IRRS-D4-001",
  "status": "FAIL",
  "severity": "S1",
  "evidence_refs": ["ev_123"],
  "observation": "QR ordering page is Chinese-only",
  "reason": "Target visitor cannot independently complete ordering",
  "confidence": 0.91,
  "needs_human_review": false
}
```

## 7.5 Stage 5 — Grounding Validation

程序检查：

- evidence_ref 是否存在；
- rule_id 是否存在；
- 状态是否合法；
- confidence 范围；
- required Evidence 是否满足。

若不满足：

- UNKNOWN；
- 或进入 Human Review；
- 禁止“补编证据”。

## 7.6 Stage 6 — Fix Suggestion

优先顺序：

1. Fix Library exact rule；
2. Fix Library similar issue；
3. LLM generated suggestion；
4. 人工确认。

## 7.7 Stage 7 — Scoring

Go backend 根据 active StandardVersion 计算。

AI Service 不返回最终 Score。

---

# 8. Prompt 与 Rule 管理规范

## 8.1 文件结构

> 2026-09-07 修订（ADR-0001 / R-09）：本节原 `ai/` 候选布局废弃。schema、规则、评测的唯一目录以 §22 仓库结构为准——契约归 `contracts/`，规则归 `standards/`，评测归 `evals/`，Prompt 归 `services/ai/prompts/`。

```text
contracts/
  json-schema/
    normalized_evidence.schema.json
    assessment.schema.json
standards/
  irrs/
    0.1.0/
      rules.yaml
services/ai/prompts/
  extraction/
    v1.md
  assessment/
    v1.md
  fix/
    v1.md
evals/
  datasets/
    golden/
```

## 8.2 Prompt 不允许

- 写在 Python 函数内部 300 行字符串；
- 无版本号修改；
- 生产中临时手改；
- Prompt 变化不跑 Eval。

## 8.3 Prompt Metadata

每次模型请求记录：

- provider；
- model；
- model_version；
- prompt_version；
- schema_version；
- input_hash；
- latency；
- token usage；
- cost estimate；
- request trace id。

敏感原始输入不一定写日志。

---

# 9. API 设计规范

Base：`/api/v1`

## 9.1 Project

```http
POST   /projects
GET    /projects
GET    /projects/{projectId}
PATCH  /projects/{projectId}
```

## 9.2 Evidence

```http
POST   /projects/{projectId}/evidence/upload-url
POST   /projects/{projectId}/evidence
GET    /projects/{projectId}/evidence
GET    /evidence/{evidenceId}
DELETE /evidence/{evidenceId}
```

## 9.3 Audit

```http
POST /projects/{projectId}/audits
GET  /projects/{projectId}/audits
GET  /audits/{auditId}
POST /audits/{auditId}/cancel
POST /audits/{auditId}/retest
```

## 9.4 Findings

```http
GET   /audits/{auditId}/findings
GET   /findings/{findingId}
POST  /findings/{findingId}/reviews
PATCH /findings/{findingId}/task
```

## 9.5 API Response

统一：

```json
{
  "data": {},
  "meta": {
    "request_id": "req_..."
  }
}
```

Error 使用 RFC 9457 Problem Details 思路：

```json
{
  "type": "https://arrivalready.dev/problems/invalid-evidence",
  "title": "Invalid evidence",
  "status": 422,
  "detail": "Unsupported MIME type",
  "instance": "/api/v1/evidence/...",
  "request_id": "req_..."
}
```

---

# 10. Async Job 设计

AI Audit 不应由一个 HTTP request 挂 60 秒等待。

## MVP

Postgres `jobs` table + worker：

```text
QUEUED → RUNNING → SUCCEEDED / FAILED
```

使用：

- `SELECT ... FOR UPDATE SKIP LOCKED`；
- idempotency key；
- max attempts；
- exponential backoff。

## 什么时候升级 Temporal / Queue

只有出现：

- 多分钟复杂 workflow；
- 跨服务补偿；
- 大规模并发；
- job recovery 成为真实问题。

之前不引入。

---

# 11. Idempotency

以下 API 必须支持 idempotency：

- create audit；
- upload complete；
- retest；
- publish guest page；
- external webhook。

Header：

`Idempotency-Key`

服务器保存 request fingerprint 和 result。

---

# 12. Authentication & Authorization

## MVP

优先采用 OIDC/OAuth2 标准兼容的外部 Identity Provider，避免团队自己发明认证协议。

后端验证 JWT：

- issuer；
- audience；
- expiry；
- signature。

## Authorization

每个资源访问同时校验：

- user；
- organization_id；
- role；
- resource ownership。

重点防范 OWASP API1 Broken Object Level Authorization。

禁止仅因为用户知道 `projectId` 就允许访问。

---

# 13. 文件上传安全

必须：

- Content-Type + magic bytes 双重检查；
- 大小上限；
- 扩展名白名单；
- 随机 object key；
- 原文件名只作为 metadata；
- Signed URL；
- 上传后扫描；
- 图片重新编码（可选）；
- PDF 解析隔离；
- 禁止执行用户文件。

---

# 14. URL 抓取 / Browser Agent 安全

这是高风险模块。

## 14.1 SSRF 防护

禁止访问：

- `127.0.0.0/8`；
- RFC1918 内网；
- link-local；
- metadata service IP；
- unix socket；
- `file://`；
- 非 http/https schema。

每次 DNS resolve 后重新校验目标 IP，防 DNS rebinding。

## 14.2 Browser Sandbox

- 独立容器；
- 非特权；
- 只读 filesystem（能做到时）；
- CPU/memory/time quota；
- 无云凭据；
- 网络 egress policy；
- 下载禁用或隔离。

---

# 15. Privacy / Data Protection

## 15.1 Data Minimization

原则：只收集验收所需数据。

门店 Walkthrough 若包含顾客：

- 默认提示避免拍摄可识别人脸；
- 可增加自动 face redaction；
- 不做身份识别；
- 不把人脸 embedding 入库。

## 15.2 Retention

MVP 建议：

- raw evidence 默认 30/90 天可配置；
- derived findings 可长期保留；
- 删除项目时提供软删除 → 延迟物理删除流程。

## 15.3 Sensitive Logging

禁止在日志直接写：

- raw document；
- token；
- password；
- full authorization header；
- 未脱敏 PII。

---

# 16. Accessibility / Internationalization

## 16.1 Accessibility

公众 Guest Page：WCAG 2.2 AA 目标。

重点：

- keyboard；
- focus visible；
- semantic HTML；
- contrast；
- alt text；
- error association；
- touch target；
- reduced motion。

## 16.2 i18n

- locale 使用 BCP 47：`en-US`, `ja-JP`, `fr-FR`；
- 不把语言和国家混为一个字段；
- 时间、数字、货币使用 `Intl`；
- UI 文案与用户内容分开；
- Translation 状态：`machine_draft / human_reviewed / published`。

---

# 17. Observability

采用 OpenTelemetry 作为 vendor-neutral telemetry 规范。

## 17.1 Trace

完整链路：

```text
Web request
 → Go API
 → Job
 → AI Service
 → Model Provider
 → DB
```

共享 trace id。

## 17.2 Metrics

### Product

- audit_started_total；
- audit_completed_total；
- finding_confirm_rate；
- retest_pass_rate。

### Technical

- request_duration_ms；
- job_queue_depth；
- ai_request_duration_ms；
- ai_schema_failure_total；
- provider_error_total；
- db_pool_usage。

### AI

- tokens；
- cost；
- unsupported_claim_rate（eval）；
- evidence_grounding_rate。

## 17.3 Logs

JSON structured：

```json
{
  "ts": "...",
  "level": "INFO",
  "service": "api",
  "request_id": "...",
  "trace_id": "...",
  "event": "audit.created",
  "project_id": "..."
}
```

---

# 18. AI Eval 工程规范

目录：

```text
evals/
  datasets/
    irrs-v0.1/
  expected/
  runners/
  reports/
```

每个 Case：

```yaml
id: qr-ordering-cn-only-001
input:
  evidence: ...
expected:
  rule_id: IRRS-D4-001
  status: FAIL
  severity: [S0, S1]
  must_reference_evidence: true
must_not:
  - claim_international_card_unsupported_without_evidence
```

CI 中：

- schema test 每次跑；
- cheap eval PR 跑；
- full model eval 在 prompt/model/rule change 时跑；
- 结果存 artifact。

---

# 19. Testing Strategy

## 19.1 Frontend

- TypeScript compile；
- component tests（关键域组件）；
- Playwright E2E；
- accessibility smoke test。

## 19.2 Go

- unit；
- repository integration against ephemeral Postgres；
- state machine test；
- authorization tests；
- OpenAPI contract test。

## 19.3 Python

- pure function unit；
- schema validation；
- provider mock；
- prompt snapshot；
- eval regression。

## 19.4 E2E Critical Paths

至少：

1. create project → upload → audit → report；
2. review finding → fix → retest → resolved；
3. unauthorized cross-org access rejected；
4. model timeout → retry/fallback；
5. malformed model output → safe failure, not corrupt data。

---

# 20. Code Quality 规范

## Frontend

- TypeScript `strict: true`；
- no `any` unless documented；
- ESLint / Biome（二选一，锁定）；
- import boundaries；
- 领域类型不在页面中重复声明。

## Go

- `gofmt`；
- `go vet`；
- `golangci-lint`；
- errors wrap with `%w`；
- context propagation；
- no global DB singleton hidden state；
- interfaces 放消费者侧，不预先抽象一切。

## Python

- Ruff format/lint；
- typing；
- Pydantic boundary validation；
- domain code 不依赖 FastAPI Request object；
- prompt/provider side effects 包装在 adapter。

---

# 21. 注释与文档规范（详细）

用户特别要求“写注释（详细）”，但详细注释不等于每行翻译代码。

## 21.1 必须写注释的位置

1. **Why**：为什么这样设计；
2. 安全边界；
3. 非直观算法；
4. 状态机；
5. 外部 API 限制；
6. AI Prompt 的业务约束；
7. Fix / workaround；
8. 数据迁移不可逆步骤。

## 21.2 不应该写的注释

```go
// userID 是用户 ID
userID := ...
```

这种注释只是让代码看起来更累。

## 21.3 推荐 Go 注释

```go
// CreateRetest creates a new immutable AuditRun instead of mutating the
// previous run. Historical scores must stay reproducible because reports
// are used to compare before/after remediation. Never reuse the previous
// audit_run_id for a retest.
func (s *Service) CreateRetest(...) { ... }
```

## 21.4 推荐 Python 注释

```python
# Evidence references are validated before persistence. The model is not
# allowed to invent a synthetic evidence id because downstream reviewers
# treat cited evidence as an auditable claim.
validated_refs = validate_evidence_refs(...)
```

## 21.5 ADR

以下变化必须写 ADR：

- DB 主类型；
- API public contract；
- Standard scoring model；
- Auth provider；
- Model provider abstraction；
- 删除/保留策略；
- 从单体拆服务。

不需要为按钮颜色写 ADR，人类文明还有别的事情要处理。

---

# 22. Repository 结构

```text
arrival-ready/
├── apps/
│   └── web/                    # Next.js
├── services/
│   ├── api/                    # Go application API
│   └── ai/                     # Python AI service/worker
├── contracts/
│   ├── openapi/
│   └── json-schema/
├── database/
│   ├── migrations/
│   └── queries/                # sqlc
├── standards/
│   └── irrs/
│       └── 0.1.0/
├── evals/
│   ├── datasets/
│   ├── runners/
│   └── reports/
├── infra/
│   ├── docker/
│   ├── compose/
│   └── otel/
├── docs/
│   ├── adr/
│   ├── prd/
│   └── runbooks/
├── scripts/
├── Makefile
├── docker-compose.yml
└── README.md
```

---

# 23. Local Development

单命令目标：

```bash
make dev
```

启动：

- Postgres；
- MinIO；
- optional Redis；
- Go API；
- AI Service；
- Web。

另外：

```bash
make test
make lint
make eval
make migrate
```

README 必须保证新成员 20 分钟内启动，而不是依靠“你微信问一下某某，他电脑上能跑”。

---

# 24. CI/CD

GitHub Actions：

## Pull Request

1. formatting；
2. lint；
3. type check；
4. unit tests；
5. migration validation；
6. contract tests；
7. frontend build；
8. Go build；
9. Python tests；
10. cheap AI eval；
11. dependency/security scan。

## Main

- build immutable image；
- image scan；
- push registry；
- deploy staging；
- smoke test；
- manual/approved production deploy。

禁止：

- SSH 上服务器 `git pull` 作为长期生产部署；
- 在服务器手改 `.env` 不记录；
- `latest` image 无版本。

---

# 25. Deployment Profiles

## 25.1 Demo / MVP

推荐：

```text
Linux VM
  ├ Web
  ├ Go API
  ├ Python AI Worker
  └ OTel Collector

Managed PostgreSQL（优先）
Object Storage
```

可以 Docker Compose 部署。

## 25.2 Production Growth

当真实负载证明需要时：

- Web CDN/Edge；
- API container platform；
- AI worker autoscaling；
- managed PostgreSQL HA；
- object storage lifecycle；
- Redis；
- independent worker queue。

## 25.3 Kubernetes Gate

满足至少 2 项才评估 K8s：

- 多服务独立扩缩容真实需要；
- 频繁发布；
- 多环境/多租户复杂；
- 有专职平台维护；
- VM/managed container 已形成明显瓶颈。

---

# 26. Security Baseline

参考 OWASP API Security Top 10 2023。

最低：

- BOLA / RBAC 测试；
- secure auth；
- input validation；
- resource quotas；
- rate limits；
- SSRF prevention；
- secrets management；
- dependency scanning；
- secure headers；
- CSRF（若 cookie auth）；
- CORS allowlist；
- signed upload/download；
- audit logs；
- backup restore test。

---

# 27. Reliability / SLO（试点）

MVP 不承诺企业级 99.99%，但内部目标：

- API availability：99.5%（试点阶段目标）；
- non-AI API P95 < 500ms；
- Audit job 成功率 > 95%；
- schema validity > 99%；
- model provider error 有 retry；
- 任何 Audit failure 不丢原始 Evidence；
- Retest 不覆盖历史。

---

# 28. Failure / Fallback

## Model timeout

- retry 1–2 次；
- exponential backoff；
- fallback provider；
- 最终标 job failed，可人工重跑。

## Structured output invalid

- provider structured mode；
- Pydantic validate；
- 一次 repair/retry；
- 仍失败 → fail safe。

## Evidence insufficient

返回：

`UNKNOWN / NEED_MORE_EVIDENCE`

绝不逼模型“必须给答案”。

## Demo Protection

比赛 Demo 可以准备经过真实运行生成的已缓存案例，但必须清楚区分：

- Live Audit；
- Cached Example。

不要在台上偷偷把静态 JSON 说成实时 AI，评委通常也会上网。

---

# 29. 性能与成本

## AI Cost Budget

每次 Audit 记录：

- tokens；
- images；
- model；
- cost。

设置：

- max evidence count；
- image resize；
- duplicate hash skip；
- extraction result cache；
- rule candidate filtering；
- 不重复把整份 PDF 发给每个 Rule。

目标：随着 Rule 增加，成本不线性爆炸。

---

# 30. Caching

可缓存：

- OCR / extraction by evidence hash；
- embeddings；
- static standard/rules；
- public Guest Page。

不可错误缓存：

- user-specific authorization；
- mutable audit status；
- signed URL。

---

# 31. 数据迁移

- 所有 schema change 必须 migration；
- 不允许生产手改表；
- migration 向前兼容优先；
- 大字段先新增 nullable → backfill → enforce；
- destructive migration 需要备份 + ADR。

---

# 32. Audit Log

记录：

- project create/update；
- evidence add/delete；
- audit start/cancel；
- finding review；
- severity edit；
- task status；
- publish；
- standard version change。

至少保存：

- actor；
- action；
- resource；
- timestamp；
- request id；
- before/after（敏感字段除外）。

---

# 33. 团队责任边界

## Product Owner

负责：

- Problem / User；
- IRRS 内容定义；
- Scope；
- UX；
- Test Cases；
- Acceptance；
- Demo；
- Customer Validation。

## AI Tech Lead

负责：

- AI Pipeline；
- Provider interface；
- Prompt/Schema；
- Eval；
- Grounding；
- Failure strategy。

## Go Full-stack / Application Lead

负责：

- Web/API integration；
- domain model；
- DB；
- state machine；
- report/retest；
- application performance。

## Java + Production / DevOps Lead

**不要求为了“Java 技术栈”建立 Java 微服务。**

负责：

- Docker；
- CI/CD；
- deployment；
- secrets；
- DB ops；
- backup；
- observability；
- security baseline；
- incident/runbook。

这是比多写一个 Spring Boot CRUD 更高价值的职责。

---

# 34. Definition of Done

一个功能只有满足以下才算 Done：

- Acceptance Criteria 通过；
- tests 通过；
- lint/type check 通过；
- error/loading/empty state；
- mobile/desktop（对应场景）；
- security considerations；
- telemetry；
- docs/API contract 更新；
- 无硬编码 secret；
- 若涉及 AI：eval case 已更新；
- 若涉及 Standard：版本已处理。

---

# 35. 48 小时 MVP 技术切片

## Slice 1（最先打通）

```text
Create Project
 → Upload Image
 → AI Structured Assessment
 → Finding
 → Evidence Display
```

必须最先跑通。

## Slice 2

```text
Human Review
 → Deterministic Score
 → Report
```

## Slice 3

```text
Fix Task
 → New Evidence
 → Retest
 → Before/After
```

## Slice 4（有时间）

```text
Guest Layer
 → Publish
 → QR
```

不要先分别做十个漂亮页面然后最后一天才发现没有 vertical slice。

---

# 36. 技术债务登记

MVP 允许有技术债，但必须显式登记：

```text
TD-001
Context: MVP uses Postgres job polling
Reason: avoid introducing queue before load exists
Exit condition: queue depth > X or workflows > Y minutes
Owner: ...
```

禁止用“以后再说”作为架构文档。

---

# 37. 参考规范与当前技术依据（检索日期：2026-09-07）

1. Next.js 16 / 16.3 security and App Router  
   https://nextjs.org/blog/next-16  
   https://nextjs.org/blog
2. React 19.2  
   https://react.dev/blog/2025/10/01/react-19-2
3. Go releases（Go 1.27.1 于 2026-09-01 发布）  
   https://go.dev/doc/devel/release
4. Python 3.14.7 current release reference  
   https://www.python.org/downloads/release/python-3147/
5. PostgreSQL 18 / 18.6  
   https://www.postgresql.org/docs/18/release-18.html  
   https://www.postgresql.org/docs/current/release-18-6.html
6. pgvector  
   https://github.com/pgvector/pgvector
7. OpenTelemetry  
   https://opentelemetry.io/docs/
8. OWASP API Security Top 10 2023  
   https://owasp.org/API-Security/
9. WCAG 2.2  
   https://www.w3.org/TR/WCAG22/
10. OpenAI Structured Outputs（JSON Schema）  
    https://openai.com/index/introducing-structured-outputs-in-the-api/
11. Gemini Structured Output  
    https://ai.google.dev/gemini-api/docs/structured-output
12. PersonaQA Platform  
    https://persona.qa/platform/
13. Synthetic Users  
    https://docs.syntheticusers.io/
14. Applause Localization Testing  
    https://www.applause.com/localization-testing/
15. qrx-joe/option-skill  
    https://github.com/qrx-joe/option-skill

---

# 38. 最终技术决策摘要

**选择：**

```text
Next.js + TypeScript
        ↓
Go Application API
        ↓
PostgreSQL + Object Storage
        ↓
Python AI Service
        ↓
Model Provider Adapter
```

核心不是“三种语言很厉害”，而是边界明确：

- TypeScript：用户体验；
- Go：确定性业务系统；
- Python：概率性 AI / Eval；
- Java/DevOps 能力：生产、交付、稳定性。

系统最重要的资产不是模型调用代码，而是：

> **IRRS Standard + Evidence Model + Issue Taxonomy + Fix Workflow + Retest History + Eval Dataset。**

如果未来 GPT、Claude、Gemini 全部升级，Arrival Ready 应该因此变得更强，而不是因此失去存在理由。

