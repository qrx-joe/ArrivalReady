# 状态迁移表（State Machines）— 契约的一部分

> 日期：2026-09-07（B03）。本文件与 `openapi/arrivalready.yaml`、`json-schema/` 同属契约，实现必须一致。
> 关闭审查项：**R-05**（结论/审核/整改三轴混用、REOPENED 无后继）、**R-06**（终态边界不完整、完成后可篡改）。
> 铁律：**任何非法迁移必须抛错（HTTP 422），任何迁移写 Audit Log（actor / before / after / reason / request_id）。**

三条状态轴相互独立、分别存储，禁止合并为一个字段：

| 轴 | 字段 | 所在 | 谁驱动 |
|---|---|---|---|
| 1. 执行/工作流 | `AuditRun.status` | run | Go job 系统 |
| 2. 检查结论 | `Finding.assessment_status` | finding | AI 候选 + 人审修订 |
| 3. 人审处置 | `Finding.review_status` | finding | 人工 |
| 4. 整改工作流 | `FixTask.workflow_status` | task | 人工（运营者） |

（轴 2/3 合并决定评分采用的 `effective_status`，见 ADR-0002 §2.5；轴 4 关联历史 Finding 但**不覆盖**它。）

---

## 1. AuditRun.status（执行轴）

```text
DRAFT → QUEUED → INGESTING → ANALYZING → REVIEW_REQUIRED → COMPLETED
非终态任意时刻 → FAILED / CANCELLED（终态）
```

| 迁移 | 条件 | 说明 |
|---|---|---|
| DRAFT→QUEUED | 输入清单冻结（证据 + 标准版本 + hash） | 冻结后 run 输入不可变（R-07） |
| QUEUED→INGESTING | worker 领取（租约 + attempt token） | 领取后 worker 崩溃由租约超时恢复（R-11） |
| INGESTING→ANALYZING | 证据校验完成 | |
| ANALYZING→REVIEW_REQUIRED | 结果通过 grounding 并原子落库 | 同事务写 Findings/关联/状态 |
| REVIEW_REQUIRED→COMPLETED | finalize：人审处置完成 + 评分冻结 | 原子冻结报告；此后只读（R-06） |
| 非终态→FAILED | job 失败且预算耗尽/不可重试 | 原始证据不丢；可建新 run 重试 |
| 非终态→CANCELLED | 用户取消或租约过期清理 | 晚到 attempt 响应无效 |

**终态边界（R-06）**：
- COMPLETED / FAILED / CANCELLED 为终态，**禁止**互转（COMPLETED 不可改 FAILED，FAILED 不可改 COMPLETED）；
- COMPLETED 后：Findings 人审、评分、证据关联全部冻结；整改活动只发生在 FixTask（轴 4）与新 run；
- 重跑、复测、更正一律产生**新 run**（复测带 `parent_run_id`）；更正历史不直接覆盖其人审记录；
- REVIEW_REQUIRED 之前允许更新执行状态与重试；finalize 是唯一进入 COMPLETED 的迁移，且必须原子（人审 + 分数 + 版本 + 证据快照一次写入）。

---

## 2. Finding.assessment_status（结论轴）

候选值来自 AI（`assessment.schema.json`），可被人审 edit 修订；`effective_status` = 评分采用的最终值：

| 来源 | 值 |
|---|---|
| AI 候选 | PASS / WARN / FAIL / UNKNOWN |
| 人审 edit 可改为 | PASS / WARN / FAIL / UNKNOWN |
| reject 且无替代判断 | UNKNOWN（**不自动转 PASS**，ADR-0002） |
| na（附理由） | 该项从适用全集排除（影响分母），与 UNKNOWN 不同 |

约束：
- PASS/WARN/FAIL 必须至少引用 1 条本 run 清单内的证据（schema 强制 + grounding 复核）；
- UNKNOWN 是合法、可持久化的结论，不是失败；
- 证据删除/到期后结论保留，证据查看返回 410 占位（R-16），hash 不作为可查看原件的替代。

---

## 3. Finding.review_status（人审轴）

```text
UNREVIEWED → CONFIRMED | EDITED | REJECTED | NA
UNREVIEWED 以外的值不可再改（终态）；更正 = 新 Review 记录 + finding 版本 +1
```

| 迁移 | 条件 |
|---|---|
| UNREVIEWED→CONFIRMED | 接受候选 |
| UNREVIEWED→EDITED | `edits` 非空；原始候选保留在 `original_candidate` |
| UNREVIEWED→REJECTED | 需 note；无替代判断时 effective_status=UNKNOWN |
| UNREVIEWED→NA | `na_reason` 必填 |
| 任意→任意（已处置后） | **禁止**；并发提交用 finding_version 乐观锁（409） |

- finalize 前置：所有 `review_required` 的 Finding 必须离开 UNREVIEWED；
- 人审记录（含 edit 前后值、理由、actor、时间）永不覆盖，只追加。

---

## 4. FixTask.workflow_status（整改轴）

```text
OPEN → ACKNOWLEDGED → FIXING → READY_FOR_RETEST → RESOLVED
                       │
                       ├────────────────→ REOPENED（复测未通过）
OPEN/FIXING → ACCEPTED_RISK
REOPENED → FIXING / READY_FOR_RETEST / ACCEPTED_RISK
```

| 迁移 | 合法 | 说明 |
|---|---|---|
| OPEN→ACKNOWLEDGED | ✅ | |
| ACKNOWLEDGED→FIXING | ✅ | |
| FIXING→READY_FOR_RETEST | ✅ | 人工声明改好；**不等于**已解决 |
| READY_FOR_RETEST→RESOLVED | ✅ | 仅当复测 run 中对应检查键 PASS（或经人审确认 PASS）；人工点击不直接产生 RESOLVED |
| READY_FOR_RETEST→REOPENED | ✅ | 复测 FAIL/UNKNOWN/缺证据（UNKNOWN 不自动解决任务） |
| OPEN→ACCEPTED_RISK | ✅ | 必填理由；≠ PASS，报告中独立列出 |
| FIXING→ACCEPTED_RISK | ✅ | 必填理由 |
| RESOLVED→* | ❌ 终态 | 复测再失败 → 新 run 的新 Finding/任务 |
| REOPENED→ACKNOWLEDGED | ❌ | REOPENED 直接回 FIXING/READY_FOR_RETEST/ACCEPTED_RISK |
| 任意→OPEN | ❌ | 无回退；错误入口用 REOPENED 语义 |
| 跳步（如 OPEN→RESOLVED） | ❌ | 必须逐态迁移 |

约束：
- 每次迁移记录 reason、actor、before/after、版本（乐观锁，409 冲突）；
- 活动状态是**投影**：历史 run 的 Finding.assessment_status 永不被任务状态覆盖（R-06/R-12）；
- 复测经人审后才更新本轴（复测 job 完成 ≠ 任务解决）。

---

## 5. Evidence.processing_status（证据轴）

```text
PENDING_UPLOAD → VALIDATING → READY
                ↘ QUARANTINED（校验/扫描失败）
任意 → DELETED（软删除/到期占位）
```

- 仅 READY 可进入 run 输入清单与 AI payload；
- QUARANTINED 为可诊断失败态（原因可见），不送 AI、不计入输入清单；
- DELETED 后查看返回 410 + 保留的 hash/元数据；清理孤儿对象时**不得**删除被任何 run 引用的材料（B06 verify）。

---

## 6. 并发与幂等不变量（跨轴）

1. 相同 `Idempotency-Key` + 相同 payload → 返回原结果；+ 不同 payload → 409；
2. worker 租约过期后，旧 attempt 的响应一律丢弃（token 不匹配）；
3. 取消后的晚到响应不落库；COMPLETED/FAILED/CANCELLED 不可取消；
4. retest/finalize/upload-complete 与并发操作竞争必须有一致性测试（B07/B10/B12 verify）；
5. 所有跨 run 比较（Diff）先校验：同标准版本 hash、同 scope/profile、检查键可对齐；否则标「不可比较」，绝不把 Finding 缺席当 PASS（R-07）。
