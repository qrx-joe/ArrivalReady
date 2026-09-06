# Arrival Ready ｜迎客验收

> 在国际访客真正到来之前，验证一家店 / 一个场馆 / 一项城市服务「发现 → 理解 → 决策 → 行动 → 支付 → 求助」的完整体验是否可用，并把问题变成可追踪、可整改、可复测的任务。

## 这是什么

- 面向**供给侧**（商户 / 场馆 / 商圈 / 文旅）的国际访客接待准备度验收系统；
- 核心闭环：**Evidence → AI 结构化评估（基于 IRRS 标准）→ 人工确认 → 确定性评分 → 整改 → 复测 → Before/After**；
- 当前处于比赛 MVP 阶段；机会验证等级 **E0–E1**（尚未完成用户验证，任何市场结论不得写成事实，见 PRD §2）。

## 文档地图

| # | 文档 | 说明 |
|---|---|---|
| 01 | [docs/01_ArrivalReady_PRD_v0.1.md](docs/01_ArrivalReady_PRD_v0.1.md) | 产品需求：机会、用户、MVP 范围、IRRS 标准、验证计划、Kill Criteria |
| 02 | [docs/02_ArrivalReady_TECH_SPEC_v0.1.md](docs/02_ArrivalReady_TECH_SPEC_v0.1.md) | 技术规范：架构原则、技术栈、领域模型、AI Pipeline、API、安全基线 |
| 03 | [docs/03_ArrivalReady_ENGINEERING_STANDARDS_v0.1.md](docs/03_ArrivalReady_ENGINEERING_STANDARDS_v0.1.md) | 工程规范：技术选型依据、同类产品制作规范、行业规范清单、注释与文档规范 |
| 04 | [docs/04_DOCUMENT_REVIEW_2026-09-07.md](docs/04_DOCUMENT_REVIEW_2026-09-07.md) | 文档审查：16 项实施缺口、原文依据与关闭标准 |
| 05 | [docs/05_EXECUTION_PLAN_v0.1.md](docs/05_EXECUTION_PLAN_v0.1.md) | 详细执行方案：16 个批次、依赖、验证门禁、提交与数据回滚策略 |
| — | [docs/requirements-matrix.md](docs/requirements-matrix.md) | 需求矩阵：P0/US → 批次映射与范围裁决状态（暂定/已定可区分） |
| — | [docs/adr/](docs/adr/README.md) | 架构决策记录（MADR）索引 |
| — | [TODO.md](TODO.md) | 当前周期任务清单（滚动更新） |
| — | [TODO_NEXT.md](TODO_NEXT.md) | 下一阶段任务队列（按依赖排序，带拉入条件） |
| — | [COMMUNICATING.md](COMMUNICATING.md) | 人机协作记录 + 决策日志 |
| — | [ADVICE.md](ADVICE.md) | AI 建议 / 不确定点 / 遗漏点清单 |
| — | [AGENTS.md](AGENTS.md) | 协作与执行约定（AI 会话与人类协作者开工前必读） |

## 快速开始

当前状态：工程基线（B04）已建立——三端空壳 + 契约 + 离线 CI 可跑；业务功能从 B05 起交付。

前置（Windows）：

- Go 1.27+（`go.mod` 含 `toolchain go1.27.1`，首次构建自动下载）、Node 24 + pnpm 10、Python 3.13 + uv、Docker Desktop（PostgreSQL 18 + MinIO）；
- 大陆网络建议设置 `GOPROXY=https://goproxy.cn,direct`（Makefile 已内置）；
- Windows 默认无 `make`：优先用 PowerShell 入口 `scripts\dev.ps1`。

```powershell
# 本地复现全部离线检查（契约/三端 lint+type+test+build，无需模型 key 与数据库）
.\scripts\dev.ps1 ci

# 启动基础设施 + 三端（各开一个终端窗口）
Copy-Item .env.example .env   # 首次
.\scripts\dev.ps1 all
```

Git Bash / WSL 下等价入口：

```bash
make contracts   # 契约正反例离线校验
make lint        # 三端静态检查
make test        # 三端测试
make infra       # PostgreSQL 18 + MinIO（compose，健康检查就绪）
make api         # Go API :8080（/healthz /readyz）
make ai          # AI Service :8100
make web         # Web :3000
```

> 注：`make dev`/`migrate` 将随 B05（身份与迁移）落地；数据卷受保护，任何情况不要执行 `docker compose down -v`。

## 维护规则

- 提交信息使用 Conventional Commits（见 docs/03 §8）；
- 决策变化必须登记 [COMMUNICATING.md 决策日志](COMMUNICATING.md)；
- 新代码必须遵守 docs/03 §6 的注释与文档规范；
- PRD / TECH_SPEC 的范围与决策改动，须同步更新对应文档版本号与决策日志。
