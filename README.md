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
| — | [TODO.md](TODO.md) | 当前周期任务清单（滚动更新） |
| — | [TODO_NEXT.md](TODO_NEXT.md) | 下一阶段任务队列（按依赖排序，带拉入条件） |
| — | [COMMUNICATING.md](COMMUNICATING.md) | 人机协作记录 + 决策日志 |
| — | [ADVICE.md](ADVICE.md) | AI 建议 / 不确定点 / 遗漏点清单 |

## 快速开始

工程基线建立后（见 [TODO.md](TODO.md) T-003），目标形态：

```bash
make dev        # PostgreSQL + MinIO + Go API + AI Service + Web
make test       # 三端测试
make lint       # 三端静态检查
make migrate    # 数据库迁移
```

## 维护规则

- 提交信息使用 Conventional Commits（见 docs/03 §8）；
- 决策变化必须登记 [COMMUNICATING.md 决策日志](COMMUNICATING.md)；
- 新代码必须遵守 docs/03 §6 的注释与文档规范；
- PRD / TECH_SPEC 的范围与决策改动，须同步更新对应文档版本号与决策日志。
