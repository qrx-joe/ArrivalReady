# 交付声明（2026-09-07，D-017）

> **按执行方案 §8 时间盒规则：deadline 提前至今日 24:00，交付目标诚实降级为技术切片。本文档是唯一权威的完成状态清单——请以此为准，勿以演示效果推断未完成的能力。**

## 已完成并验证（有自动化测试与实跑证据）

| 能力 | 验证方式 |
|---|---|
| 项目创建 / 证据上传（presign → 服务端 magic bytes + sha256 校验 → READY/QUARANTINED） | Go 集成测试 + 真实 MinIO 实跑 |
| 组织隔离与审计日志（跨租户读写全部 fail closed） | 真实 PG 集成测试 |
| 可恢复审计任务（SKIP LOCKED 领取 / 租约 / 崩溃恢复 / 取消晚到丢弃 / 原子落库） | 真实 PG 集成测试 8 项 |
| StepFun 真实模型评估（D-016）+ 证据内联 base64 传输 | 真实调用冒烟 + 全流程 E2E（38–44s） |
| 前端工作流：登录 → 创建项目 → 上传 → 审计 → Evidence Viewer（原图 + bbox 定位 + 观察/推断 + 待人审标识） | Playwright E2E 2/2 |
| **人审四操作**（confirm / edit / reject / NA 需理由）与不可覆盖语义 | API + 集成路径 |
| **确定性评分与报告冻结**（ADR-0002 / D-018，手算样例单测钉死：62.50 / 83.33-partial / 83.33-NA） | 单元测试 + finalize 实跑 |
| **整改任务状态机**（OPEN→…→RESOLVED / ACCEPTED_RISK 需理由 / 非法迁移拒绝） | API 实跑 |
| **复测 + Before/After Diff**（按 rule key 对齐；缺席≠PASS） | API 实跑 + E2E |
| 离线 Eval runner（5 类 fixture：合法/缺证据/伪造引用/注入/供应商失败） | CI contracts 作业 |
| 离线 CI（Go/AI/Web/contracts 四作业，无模型 key 可跑） | GitHub Actions 全绿 |

**真实模型全流程演示结论（2026-09-07 实录）**：上传自制英文菜单 fixture 后，10 条规则评估落库——有证据项判定准确（语言可达 PASS、价格透明 PASS、支付标识 PASS），**无证据项全部诚实 UNKNOWN**（门头/扫码页/人工支付确认），未编造任何结论（No Fake Certainty 落地）。

## 未完成（显式声明，不冒充完成）

| PRD P0 项 | 状态 | 原因 |
|---|---|---|
| P0-2 URL 自动抓取 | **未实现**（D-021：已批准，2–3 人日工作量超出时间盒） | SSRF 隔离执行器未交付；URL 来源登记已具备 |
| P0-2 PDF / 文本解析 | 未实现（B13 未进入） | 图片链路优先 |
| 20 例 Golden Set + 评测门槛 | 未完成 | 真实素材与访谈未执行（T-005/T-006） |
| 完整比赛 MVP 声明 | **不成立** | 以上任一未达即不满足 PRD §26 完整验收 |

P1 未实现：Guest Page、QR、报告导出、多语言 Profile、团队协作。

## 已知限制（如实登记）

- 真实模型全流程依赖图片内联 base64（≤10MB）；云端部署时建议切回预签名 URL + 公网存储（契约已支持两种，schema 注释说明）
- 本机 Docker Desktop 引擎故障，compose 探活验证由 ephemeral PG/MinIO 进程等价替代；T-003 尾项待 Docker 恢复后补验
- E2E 的 CI job 未接线（本地全栈已验证；后续补 CI 编排）
- OIDC 为开发测试身份模式（真实 IdP 配置待接入；生产前必须切换，见 AGENTS.md §7）
- E2E 里 fake 与 real 两次说明：fake 路径 CI 可复现（离线绿），real 路径依赖真实 key 与网络（已单独标注，两次实跑通过）

## 启动指南（评委 / 新成员）

```powershell
# 前提：Windows + Go 1.27 + Node 24/pnpm + Python 3.13/uv + PostgreSQL + MinIO
# 1. 配置模型 key（已由 PO 配置）
#    services/ai/.env → MODEL_API_KEY / MODEL_BASE_URL / MODEL_ID=step-1o-turbo-vision
# 2. 启动全栈
powershell -File scripts\dev-e2e.ps1
# 3. 打开 http://localhost:3000 → 开发者登录 → 创建项目 → 上传菜单图 → 启动审计
```

回滚：`git revert` 对应批次 PR 的 merge commit；数据库迁移向前修复为默认策略（执行方案 §7.3）。
