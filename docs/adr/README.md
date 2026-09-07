# ADR 索引（Architecture Decision Records）

> 格式：MADR（docs/03 §4.4）；触发清单见 TECH_SPEC §21.5。
> 规则：已接受的 ADR 不修改；推翻时新增一篇并标注 supersedes。新 ADR 编号按本表顺延，落盘前先在本表登记。

| 编号 | 标题 | 状态 | 日期 | supersedes |
|---|---|---|---|---|
| [0001](0001-canonical-repository-layout.md) | 规则、契约、评测与 Prompt 的唯一目录来源 | 已接受 | 2026-09-07 | TECH_SPEC v0.1 §8.1 候选布局 |
| [0002](0002-scoring-and-review.md) | 确定性评分与人审语义 v1 | 已接受（D-018） | 2026-09-07 | — |
| [0003](0003-migration-tool.md) | 迁移工具选用 golang-migrate | 已接受 | 2026-09-07 | —（关闭 S-2 二选一遗留） |
