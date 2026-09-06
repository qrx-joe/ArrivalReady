# TODO NEXT ｜ 下一阶段任务队列

> **2026-09-07 审查补充**：队列与 [执行方案](docs/05_EXECUTION_PLAN_v0.1.md) §9 对照使用。N-03 的最小 Eval runner 应提前至首个模型批次，N-04 安全基础应提前至真实素材入系统前；URL 的 P0/P1 冲突已在 B01 处理（[需求矩阵](docs/requirements-matrix.md) §4.2 / D-011 草案：登记与抓取分离）。以下为原队列，尚未视为已完成或已正式调整范围。

> **维护规则**
> - 这里是「排好队但尚未开始」的任务，按建议执行顺序排列；
> - 拉入 [TODO.md](TODO.md) 时复制条目并补 Owner 与截止点；
> - 每项必须写明**拉入条件（Trigger）**——防止凭感觉抢跑，也防止该拉的不拉；
> - 底部有「明确不排队的项」，往里加东西前先看 TECH_SPEC §2.1（不为 hackathon 制造分布式系统）。

---

## 队列（按建议执行顺序）

### N-01 ｜ Slice 2：人工确认 → 确定性评分 → Report
- **内容**：Review API（confirm / reject / edit / na）+ Scoring Service（Go 侧，按 StandardVersion 权重确定性计算）+ 报告页（维度分 / PASS-WARN-FAIL / Critical 列表）；
- **依赖**：T-004（IRRS 规则集）、T-009（Slice 1）；
- **拉入条件**：Slice 1 端到端跑通，且每条 Finding 都有真实 Evidence 引用；
- **对应**：PRD P0-4 / P0-5；TECH_SPEC §7.7（AI 不返回分数）。

### N-02 ｜ Slice 3：Fix Task → 新证据 → Retest → Before/After
- **内容**：Finding 状态机落库（含非法迁移拦截）、FixTask 分配、Retest 创建**新的不可变** AuditRun、Diff 视图；
- **依赖**：N-01；
- **拉入条件**：Slice 2 的评分与报告可用；
- **对应**：PRD P0-6 / P0-7。**这是比赛 Demo 脚本（PRD §25）的必要环节，砍 P1 也不能砍它。**

### N-03 ｜ Eval Pipeline v0
- **内容**：schema test 进 CI；golden runner 本地可跑；每次模型请求记录 cost / latency / prompt_version；
- **依赖**：T-006（Golden Dataset 素材）、T-008（Provider Adapter）；
- **拉入条件**：首批 20 个 golden case 就绪且与规则集版本对齐；
- **对应**：TECH_SPEC §18 / PRD §16。

### N-04 ｜ 认证与安全基线（引入真实商户数据前**必须**完成）
- **内容**：OIDC 登录（外部 IdP）、organization 归属校验（防 BOLA，OWASP API1）、Signed URL 下载、上传 magic bytes 校验、API rate limit；
- **依赖**：T-003；
- **拉入条件**：Slice 1 打通后、任何真实商户材料入库之前；
- **对应**：TECH_SPEC §12 / §13 / §26。

### N-05 ｜ 可观测性基线（可与 N-01 并行，不阻塞主线）
- **内容**：slog JSON 结构化日志、request_id / trace_id 贯通三层、OTel SDK 接入、核心 metrics（audit_started / audit_completed / ai_schema_failure_total）；
- **依赖**：T-003；
- **拉入条件**：Slice 2 开发期间顺手做；不为此推迟 Slice 3；
- **对应**：TECH_SPEC §17。

### N-06 ｜ Demo 包装（比赛向）
- **内容**：Demo Script 排练（PRD §25 的 90 秒版本）；缓存案例与 Live Audit 的**明确 UI 标识**（TECH_SPEC §28）；90 秒离屏录屏兜底；现场网络应急预案；
- **依赖**：N-02；
- **拉入条件**：比赛时间线确定后倒排；**Demo 前 48 小时必须开始，此后冻结功能开发**；
- **对应**：ADVICE A-7。

### N-07 ｜ P1 功能池（按剩余时间从上往下取，做完一个再取下一个）
- Guest Page + QR（过渡层，非产品本体，PRD §13.2）；
- URL 证据抓取（执行批次 B14，**含 SSRF 防护全部要求**，TECH_SPEC §14——做不全就别做这个功能；URL 来源登记与自动抓取已分离定义，登记不依赖抓取，见[需求矩阵](docs/requirements-matrix.md) §4.2）；
- 报告导出（PDF）；
- 英 / 日目标访客 Profile；
- 团队成员与任务负责人（多用户协作）；
- **拉入条件**：Slice 1–3 完成且演示主线稳定，剩余时间 > 2 天。

---

## 明确不排队的项（防 scope creep）

| 不做 | 触发条件（满足才重新评估） |
|---|---|
| Kubernetes / 服务拆分 | TECH_SPEC §25.3 的 K8s Gate（至少满足 2 项） |
| Kafka / 独立队列 | job 积压或多分钟级 workflow 成为真实问题（TECH_SPEC §10） |
| 视频 Evidence / 多门店 Benchmark / API-Webhook | P2，比赛后（PRD §10.2–10.3） |
| PRD §10.4 Won't Have 全部项 | 除非 PRD 本身改版并记入决策日志 |
