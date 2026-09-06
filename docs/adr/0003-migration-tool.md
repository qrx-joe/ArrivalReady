# ADR-0003：数据库迁移工具选用 golang-migrate

- 状态：已接受
- 日期：2026-09-07
- 决策人：AI（工程约定；PO/Lead 异议时新增 ADR supersedes）
- 关闭问题：S-2 遗留「golang-migrate vs Atlas 未锁定（启动后二选一）」（TECH_SPEC §3.2）
- 决策日志：D-015

## 背景与问题

TECH_SPEC §3.2 列出 golang-migrate 或 Atlas 二选一，S-2 会话明确「项目启动后锁定」。B05 需要第一组迁移，必须先定工具。

## 决策

选用 **golang-migrate**（`golang-migrate/migrate/v4`）：

1. 以 Go 库形式嵌入服务与测试（iofs/文件源），`go test` 可直接对真实临时 PostgreSQL 跑「空库迁移 / 升级 / down 验证」，不依赖外部 CLI；
2. 社区最广泛、行为朴素（纯 SQL up/down 文件），符合「成熟优先于新颖」（docs/03 §1）；
3. Atlas 的声明式 schema 与 replay 能力当前规模用不上（TECH_SPEC §2.1：不为黑客松制造工具复杂度）。

配套约定：

- 迁移文件：`database/migrations/NNNN_name.up.sql / .down.sql`（成对、只追加，已应用的迁移文件不可修改）；
- 服务启动**不自动迁移**（生产安全边界）；`cmd/migrate` 子命令提供 up/down/status，CI 与本地测试显式调用；
- 破坏性 down 禁止对真实库自动执行（执行方案 §7.3）——down 文件只为测试与回滚演练存在。

## 后果

- B05 起所有 schema 变更走 `database/migrations/`；
- 镜像/部署需携带迁移目录（部署批次的 runbook 项）。

## 验证

- `go test`（集成）：空库 up 全部版本 → down 逐步 → 再 up，结果一致；
- `cmd/migrate status` 输出与库内版本一致。
