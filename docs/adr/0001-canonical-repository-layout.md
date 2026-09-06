# ADR-0001：规则、契约、评测与 Prompt 的唯一目录来源

- 状态：已接受
- 日期：2026-09-07
- 决策人：AI（工程约定；依据 docs/03 §5.1 既定依赖方向。PO 有异议时新增 ADR supersedes 本篇，不修改本篇）
- 关闭问题：[文档审查记录 R-09](../04_DOCUMENT_REVIEW_2026-09-07.md)
- 决策日志：D-010

## 背景与问题

TECH_SPEC v0.1 存在两套候选目录：§8.1 把 schema、rules、evals 放在 `ai/` 目录下，而 §22 仓库结构又定义了顶层 `contracts/`、`standards/`、`evals/`。TODO、执行方案与工程规范引用的都是后者。若不固定唯一来源，会出现两份事实源，且规则/评测被错误地归属为 Python AI 服务的私产，与「`contracts/` 是三端唯一共享物」（docs/03 §5.1）的依赖方向冲突。

## 决策

采用 TECH_SPEC §22 的顶层目录为唯一来源，废弃 §8.1 的 `ai/` 候选布局：

| 内容 | 唯一位置 | 废弃候选 |
|---|---|---|
| OpenAPI / JSON Schema / 共享 fixture | `contracts/`（openapi、json-schema、fixtures） | ~~`ai/schemas/`~~ |
| IRRS 规则（按 SemVer 目录） | `standards/irrs/<x.y.z>/` | ~~`ai/rules/`~~ |
| Golden dataset / runner / 报告 | `evals/`（datasets、runners、reports） | ~~`ai/evals/`~~ |
| Prompt（版本化文件） | `services/ai/prompts/` | ~~`ai/prompts/`~~ |

理由：

- 契约被 Web/Go/Python 三端共同消费，必须放在无依赖方向的共享层；
- 规则是产品核心资产而非 AI 服务私产：Go 评分服务与 Eval runner 都要读取同一份规则，放 `services/ai/` 会造成跨服务反向依赖；
- Prompt 只被 Python 消费，放在 `services/ai/` 内部，避免顶层目录膨胀；
- 评测数据与报告有独立生命周期（脱敏 manifest、版本对齐），独立顶层目录便于不提交原始素材。

## 后果

- TECH_SPEC §8.1 已按本决策改写，§22 不变；
- B02（规则）、B03（契约）、B08（Prompt/runner）落盘时直接使用上述路径；
- 当前仓库无代码与已发布制品，无迁移成本。

## 验证

- B02/B03/B08 的 verify 项中检查实际落盘路径与本表一致；
- CI 建立后（B04）以 `contracts/` 为 lint 与 fixture 校验对象。
