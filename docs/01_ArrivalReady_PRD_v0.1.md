# Arrival Ready｜迎客验收 — 产品需求文档（PRD）

> 文档版本：v0.1（Opportunity Validation / MVP）  
> 日期：2026-09-07  
> 状态：**候选项目，尚未完成 E2/E3 需求验证，不应把市场假设写成已验证事实**  
> 同步注记（2026-09-07，B01）：需求→批次映射与范围裁决状态见 [需求矩阵](requirements-matrix.md)；P0-2 中 URL「来源登记」与「自动抓取」的拆分及任何 P0 范围变化均为草案（决策 D-011），未经 PO 确认不改变本文件 P0 定义。  
> 赛道：沪潮涌江与远航｜AI 城市内容与品牌出海实验室  
> 产品负责人：Product Owner  
> 技术负责人：AI Tech Lead / Application Lead / Production Lead  

---

## 0. 文档目的

本 PRD 用于定义 Arrival Ready（中文暂定“迎客验收”）的产品机会、目标用户、核心价值、功能边界、AI 能力、验证方案、商业闭环、运营方式、指标和风险。

本文遵循一个基本原则：

> **先验证问题，再自动化解决方案。模型只是系统中的一个推理节点，而不是整个产品。**

本项目参考 `qrx-joe/option-skill` 的机会决策方法：证据不足时不依赖“主观高分”推进开发；E0–E4 证据阶梯优先于总分；口头意愿不等于真实需求；在行动和付费证据出现前，优先采用人工/服务式验证。

---

# 1. Executive Summary

## 1.1 一句话介绍

**Arrival Ready 帮助商户、场馆和城市品牌在国际访客真正到来之前，验证“从发现、理解、选择、行动、支付到求助”的完整体验是否可用，并把问题变成可追踪、可整改、可复测的任务。**

## 1.2 核心命题

上海已经存在面向国际游客的多语言旅行信息、推荐和支付类服务。Arrival Ready 不再做一个“国际游客 AI 导游”，而是站在供给侧解决另一个问题：

> **当国际游客真的走进一家店、一个场馆、一项城市服务时，这个服务本身准备好了吗？**

## 1.3 产品不是

Arrival Ready **不是**：

- AI 菜单翻译器；
- AI 城市导游；
- 外语文案生成器；
- 让 LLM “扮演一个外国人”并输出建议的聊天框；
- 通用网站 UX synthetic-user 测试平台的简单复制；
- “做一个多语言网页”生成器。

## 1.4 产品真正交付的结果

对于一个待验收对象（门店 / 场馆 / 页面 / 线下服务流程），系统交付：

1. **Readiness Score**：分维度准备度，而非模糊总评；
2. **Findings**：具体问题；
3. **Evidence**：每个问题对应证据；
4. **Severity**：严重程度与影响；
5. **Fix Plan**：可执行整改；
6. **Owner & Status**：负责人和整改状态；
7. **Retest**：整改后的重新验收；
8. **History**：版本变化与证据链；
9. （可选）**Guest Layer**：快速生成面向国际访客的补充页面/二维码，作为无法即时改造原系统时的过渡层。

---

# 2. Opportunity Status：先承认我们还不知道什么

## 2.1 当前证据等级

按照 option-skill 的 E0–E4 方法，当前项目应标记为：

**E0–E1：合理假设 + 市场信号，尚未得到足够的具体近期事件和已发生代价。**

当前可以确认的是：

- 上海正在持续建设国际游客多语言、支付、信息与旅行服务能力；
- 数字本地化、真人本地化测试、Synthetic User、网页 UX 自动测试已经形成成熟或快速增长的产品类别；
- 这证明“跨市场体验”和“体验验证”是现实类别，但**不能证明 Arrival Ready 的具体需求已经成立**。

当前尚未确认：

- 上海中小商户是否频繁因国际访客流程产生实际订单损失或人力成本；
- 谁最痛：单店老板、连锁总部、商圈、文旅单位还是场馆；
- 他们现有解决方法是什么；
- 是否已经为此花钱；
- 他们是否愿意提供资料、接受整改、重新测试；
- 谁拥有预算和采购权。

## 2.2 E2 痛点句式（待访谈验证）

目标不是让用户认同下面这句话，而是寻找真实事件是否支持它：

> **[面向国际访客的商户/场馆运营者] 在 [国际访客到店并尝试完成消费或服务] 时，希望 [访客能独立理解、选择、行动和完成交易]，但因 [语言、数字流程、文化语境、支付、求助路径或线下标识只按本地用户设计]，产生 [额外人工协助、交易中断、投诉、低转化或重复改造成本]。**

只有出现具体近期事件 + 可描述代价，才视为达到 E2。

---

# 3. 用户、受益者与买方

## 3.1 核心用户角色

### U1：门店 / 场馆运营人员（Primary User）

典型任务：

- 检查现有国际访客体验；
- 上传门店、菜单、二维码、网页、流程等材料；
- 查看问题与证据；
- 分配整改任务；
- 修改后重新验收；
- 向负责人展示整改结果。

### U2：国际访客（Beneficiary）

他们通常不会主动成为 Arrival Ready 后台用户，但会直接受益：

- 更容易找到入口；
- 更容易理解商品与服务；
- 更容易完成下单/购票/支付；
- 更容易获得帮助；
- 更少因为“看得懂单词却不知道怎么做”而放弃。

### U3：运营负责人 / 店主 / 连锁总部（Buyer / Decision Maker）

关注：

- 是否影响转化、服务效率、投诉率；
- 多店如何统一标准；
- 是否能形成可审计的验收结果；
- 整改成本和 ROI；
- 是否可批量部署。

### U4：商圈 / 文旅 / 场馆管理方（Potential Enterprise Buyer）

关注：

- 批量评估多个商户/场馆；
- 统一 Readiness Standard；
- 展示城市国际服务能力改善；
- 形成整改项目清单和可复查证据。

---

# 4. Jobs To Be Done（JTBD）

## 4.1 Functional Job

当我要接待更多国际访客时，我希望知道现有服务中**哪些具体步骤会让他们失败**，并得到可执行整改和复测结果，而不是只得到一份泛泛的“国际化建议”。

## 4.2 Emotional Job

我希望在真实顾客投诉或流失之前发现问题，而不是等到现场尴尬地临时翻译、解释或补救。

## 4.3 Social Job

作为运营方，我希望能够向老板、总部或管理机构证明：

> “我们不是做了一张英文菜单，而是完成了一套可验证的国际访客体验整改。”

---

# 5. 当前用户旅程与问题发现框架

Arrival Ready 采用端到端 Journey，而不是孤立地检查“翻译质量”。

## 5.1 国际访客服务 Journey

1. **Discover / 发现**
   - 能否找到？
   - 地图、名称、营业时间、入口是否明确？

2. **Understand / 理解**
   - 能否知道这是什么地方？
   - 商品/服务是什么？
   - 文化概念是否只有直译没有解释？

3. **Decide / 决策**
   - 能否比较选项？
   - 是否知道价格、份量、适用人群、过敏原等？

4. **Act / 行动**
   - 能否排队、预约、点单、购票、使用设备？
   - 扫码后是否进入仅中文系统？

5. **Pay / 支付**
   - 是否清楚可用支付方式？
   - 国际卡/现金/移动支付路径是否明确？

6. **Recover / 求助与恢复**
   - 出错后找谁？
   - 退款、取消、修改、投诉怎么做？

7. **Remember / 离店与延续**
   - 是否知道品牌名、购买渠道、复购方式？
   - 是否可分享和再次找到？

---

# 6. 竞争分析

## 6.1 竞争不是只看“长得像我们的产品”

需要同时比较五类替代方案：

1. 通用大模型；
2. 本地化 / 翻译平台；
3. Synthetic User / 网页体验测试；
4. 真人本地化测试与 Mystery Shopping；
5. 城市游客服务平台；
6. 用户“不解决 / 临时处理”的现有做法。

## 6.2 竞品矩阵

| 类别 | 代表 | 强项 | 与 Arrival Ready 的重叠 | 我们必须形成的差异 |
|---|---|---|---|---|
| 通用 AI | ChatGPT / Claude / Gemini | 图像理解、翻译、分析、建议、网页生成 | 很高 | 标准、证据、任务闭环、历史、复测、真实业务流程 |
| Localization | Phrase / Lokalise | 多语言内容、翻译工作流、审校 | 内容层高度重叠 | 不以“翻译内容”为终点，验收线下+数字+交易流程 |
| Synthetic User | PersonaQA / Synthetic Users | 真实浏览器旅程、网页摩擦、报告 | 数字 Journey 高度重叠 | 线下物理空间、商户资料、支付/服务 SOP、跨文化验收标准 |
| 真人测试 | Applause | 全球真人、语言/文化/货币测试 | 高质量跨市场验证 | 更低成本的 SMB 初筛 + 自动整改闭环；必要时可升级真人复核 |
| 城市游客平台 | Meet China / 沪小游 | 游客端信息、推荐、多语言、支付 | 服务同一终端用户 | 我们服务供给侧“是否准备好”，不是再做 C 端导游 |
| 人工临时方案 | 店员翻译、英文菜单、朋友帮忙 | 便宜、立即可用 | 解决局部问题 | 把隐性问题结构化、可重复、可批量、可复测 |

## 6.3 关键竞品事实

- PersonaQA 强调“真实浏览器 Journey + Evidence-backed findings + 可重复比较”，说明单纯 AI Persona 已经不是差异化；
- Synthetic Users 也已能让 AI persona 操作真实 Chromium 并输出 confusion points、截图与报告；
- Applause 用 200+ 国家/地区真人测试覆盖语言、文化、货币等本地化维度；
- Lokalise 已将“翻译 → 审核 → 工作流”产品化；
- 上海 Meet China 和“沪小游”持续增强国际游客侧的信息、推荐、多语言和支付能力。

因此 Arrival Ready **不能以“AI 模拟外国游客”作为核心卖点**。

---

# 7. 通用 AI 80% 替代测试

## 7.1 淘汰规则

如果用户将同样材料交给 ChatGPT / Claude，并通过聊天框即可完成核心价值的 80% 以上，则该功能不应成为产品核心。

## 7.2 功能级判断

| 功能 | 通用 AI 替代率 | 结论 |
|---|---:|---|
| 菜单翻译 | >90% | 不做核心 |
| 文化概念解释 | 70–90% | 只作为能力模块 |
| 看门头/菜单并给建议 | >80% | 不做核心 |
| 生成多语言 Guest Page | 60–80% | 只能作为输出，不是壁垒 |
| 标准化验收 + Evidence | 30–50% | 核心 |
| Issue Taxonomy + 确定性评分 | <40% | 核心 |
| 整改任务分配/状态/历史 | <20% | 核心 |
| Retest + Regression | <20% | 核心 |
| 多门店 Benchmark | <10%（无私有数据时） | 长期壁垒 |
| 实际整改效果数据 | <10% | 长期壁垒 |

## 7.3 产品原则

> **模型可以替换；标准、证据、工作流、历史和真实结果不可随模型一起消失。**

---

# 8. 价值主张

## 8.1 对单店/场馆

从：

> “我好像做了英文版，应该可以吧。”

变成：

> “我知道在哪一步 FAIL、为什么 FAIL、证据在哪里、谁负责改、改完是否通过。”

## 8.2 对连锁/商圈

从零散的人工巡检升级为：

- 同一套标准；
- 同一套问题分类；
- 可量化对比；
- 可批量整改；
- 可追踪复测。

## 8.3 对国际访客

减少“文字翻译正确但流程仍然不可用”的体验摩擦。

---

# 9. 产品原则

1. **Evidence First**：每个重要结论必须尽可能绑定证据；
2. **Deterministic Score**：最终分数由规则引擎计算，LLM 不直接自由打总分；
3. **AI Suggests, Human Owns**：AI 可以提出问题和整改，关键发布由人确认；
4. **Fix > Report**：报告不是终点，整改和复测才是闭环；
5. **Model Agnostic**：模型供应商可替换；
6. **No Fake Certainty**：低置信度必须显式表达；
7. **Mobile First**：线下巡检和访客页面优先移动端；
8. **Privacy by Default**：尽量避免处理不必要个人数据；
9. **Pilot Before Platform**：先服务式验证，再规模化自动化；
10. **Scope Discipline**：MVP 不做 CRM、支付系统和完整旅游平台。

---

# 10. MVP 范围

## 10.1 P0 — 必须完成

### P0-1 创建验收项目

用户可以：

- 创建对象（店铺/场馆）；
- 填写名称、类型、地址/场景；
- 选择目标访客语言/市场；
- 选择本次验收范围。

### P0-2 上传 Evidence

支持：

- 图片；
- PDF；
- URL；
- 文字说明；
- 最短版本可先不支持视频。

Evidence 必须保存：

- 文件/URL 来源；
- 上传时间；
- 来源类型；
- 关联 Journey Step；
- 处理状态。

### P0-3 自动分析

AI Pipeline 输出结构化 finding：

- rule_id；
- issue_type；
- journey_stage；
- severity；
- title；
- description；
- evidence_refs；
- confidence；
- recommended_fix；
- model_metadata。

### P0-4 Readiness Report

界面至少展示：

- 维度评分；
- PASS / WARN / FAIL；
- Critical Findings；
- Evidence；
- 整改建议。

### P0-5 人工确认

运营者可以：

- Confirm；
- Reject AI finding；
- Edit；
- Mark N/A；
- 填写备注。

### P0-6 整改状态

Finding 状态：

`OPEN → ACKNOWLEDGED → FIXING → READY_FOR_RETEST → RESOLVED / ACCEPTED_RISK`

### P0-7 Retest

整改后重新上传或重新抓取证据，生成新的 Audit Run；保留上一版本结果，支持 Before / After。

---

## 10.2 P1 — 强烈建议

- Guest Page / Guest Layer；
- QR Code；
- URL 浏览器自动任务检查；
- 团队成员与任务负责人；
- 报告导出；
- 英文/日文两种目标访客 Profile；
- 基础 Analytics；
- Prompt / Rule / Model 版本显示。

## 10.3 P2 — 比赛后

- Walkthrough Video 时间轴证据；
- 多门店 Portfolio；
- Benchmark；
- 真人国际用户复核 marketplace；
- 自动监测 URL 变化；
- 连锁总部模板；
- API / Webhook；
- 组织级 RBAC；
- 商圈批量任务；
- 真实转化/投诉指标回传。

## 10.4 Won't Have（明确不做）

MVP 不做：

- 自建支付系统；
- 酒店/机票/旅行路线；
- 完整 CRM；
- 社交社区；
- 100 种语言；
- 自动修改第三方小程序；
- “AI 认证证书”式无法证明的权威标签；
- 对真实国际游客进行未经同意的画像或跟踪。

---

# 11. Internal Readiness Standard（IRRS）v0.1

> 这是内部产品标准，不对外宣称为政府/行业认证。

## 11.1 Dimension

| Code | Dimension | Weight（MVP 假设） |
|---|---|---:|
| D1 | Discoverability | 10% |
| D2 | Comprehensibility | 20% |
| D3 | Decision Support | 15% |
| D4 | Task Completion | 20% |
| D5 | Payment Readiness | 15% |
| D6 | Recovery & Help | 10% |
| D7 | Cultural Context / Continuity | 10% |

> 权重必须版本化；未经过实证前仅作 MVP 计算假设。

## 11.2 Severity

- **S0 Blocker**：核心任务无法完成；
- **S1 Critical**：高概率导致放弃/重大误解；
- **S2 Major**：明显增加摩擦或人工协助；
- **S3 Minor**：体验不佳但可完成；
- **Info**：建议项，不扣分。

## 11.3 Score 原则

禁止 LLM 直接生成“83 分”。

建议：

1. LLM / Rule Engine 判断每个检查项状态；
2. 人工可复核；
3. Deterministic Scoring Service 根据 rule weights 计算；
4. 每个 Audit 绑定 `standard_version`；
5. 标准变化不能直接覆盖历史结果。

---

# 12. 核心用户流程

## 12.1 Flow A：首次验收

```text
创建项目
  ↓
选择目标访客
  ↓
上传/抓取 Evidence
  ↓
AI 预处理
  ↓
Rule + AI Assessment
  ↓
Finding + Evidence
  ↓
人工确认
  ↓
Readiness Report
```

## 12.2 Flow B：整改

```text
打开 Critical Finding
  ↓
查看 Evidence
  ↓
查看 Suggested Fix
  ↓
Assign Owner
  ↓
执行整改
  ↓
Ready for Retest
```

## 12.3 Flow C：复测

```text
Retest
  ↓
新 Audit Run
  ↓
重新收集 Evidence
  ↓
新 Findings
  ↓
Diff
  ↓
Resolved / Still Failing
```

## 12.4 Magic Moment

MVP 必须刻意设计一个明显瞬间：

> **用户第一次看到“系统不仅说有问题，还指出具体证据、规则、影响，并能在整改后看到 FAIL → PASS”。**

这比“AI 输出了一段漂亮建议”更重要。

---

# 13. 页面信息架构

## 13.1 运营端

- `/`
  - Landing
- `/dashboard`
  - Projects
- `/projects/new`
- `/projects/[id]`
  - Overview
  - Evidence
  - Audit Runs
  - Findings
  - Tasks
  - Guest Layer（P1）
- `/audits/[id]`
  - Score
  - Journey Map
  - Findings
  - Evidence
  - Diff
- `/findings/[id]`
  - Evidence Viewer
  - Rule
  - AI Explanation
  - Human Review
  - Fix
  - Retest
- `/settings`

## 13.2 Guest Page（P1）

- What is this place?
- What can I do here?
- How to order / visit?
- How to pay?
- What should I know?
- Need help?

Guest Page 是整改手段之一，不是 Arrival Ready 本体。

---

# 14. User Stories 与验收标准

## US-01 创建项目

**作为运营人员**，我希望创建一个门店验收项目，以便保存该门店的所有证据和历史验收。

Acceptance：

- 必填字段校验；
- 项目创建后有唯一 ID；
- 记录创建人/时间；
- 可以删除/归档，但默认不物理删除历史 Audit。

## US-02 上传证据

**作为运营人员**，我希望上传图片和 URL，以便系统分析真实服务材料。

Acceptance：

- 文件格式白名单；
- 大小限制；
- 上传进度；
- 失败重试；
- Signed URL；
- 每个 Evidence 可追溯到原始来源。

## US-03 查看 Finding

**作为运营人员**，我希望看到问题对应的证据，而不是只相信 AI。

Acceptance：

- Finding 至少一个 evidence_ref，若没有必须标为 `insufficient_evidence`；
- 显示 AI confidence；
- 显示 rule_id 和 standard version；
- 用户可以 Reject。

## US-04 复测

**作为负责人**，我希望整改后重新测试，确认问题真的解决。

Acceptance：

- Retest 创建新的 audit_run；
- 旧结果不可覆盖；
- 支持 Diff；
- Finding 可以 resolved / reopened。

---

# 15. AI 产品需求

## 15.1 AI 在系统中的职责

AI 负责：

- OCR / Visual Understanding；
- 资料结构化；
- 场景实体抽取；
- 按规则判断潜在问题；
- 证据摘要；
- 跨文化解释；
- 整改建议草案；
- 多语言 Guest Content 草案。

AI **不负责**：

- 自由决定总分；
- 无证据地断言法律/支付/过敏安全事实；
- 自动发布高风险内容；
- 覆盖人工判断；
- 将“模型自信”当成事实可信度。

## 15.2 AI 输出必须结构化

所有核心 AI 输出必须遵循 JSON Schema；禁止后端通过正则解析自然语言 Markdown 作为生产契约。

## 15.3 Human in the Loop

以下 Finding 默认要求人工确认：

- 支付可用性；
- 过敏原/食品安全；
- 法规与合规；
- 无障碍安全；
- 营业/退款政策；
- 涉及品牌承诺的事实。

---

# 16. AI Evaluation（Eval）

## 16.1 Golden Dataset

MVP 至少建立 20 个测试 Case：

- 8 个明确 FAIL；
- 5 个 WARN；
- 5 个 PASS；
- 2 个证据不足，应输出 UNKNOWN / NEED_REVIEW。

每个 Case 包含：

- 输入 Evidence；
- 预期 rule_id；
- 预期 status；
- 允许 severity 范围；
- 必须引用的 Evidence；
- 禁止出现的断言。

## 16.2 AI Metrics

- Finding Precision；
- Critical Finding Recall；
- Evidence Grounding Rate；
- Unsupported Claim Rate；
- Schema Validity Rate；
- Consistency / Regression Pass Rate；
- Cost per Audit；
- P50/P95 Latency。

## 16.3 Release Gate

任何 Prompt / Model / Rule 版本变化前：

1. 跑 Golden Dataset；
2. 对比上一版本；
3. Critical Recall 不得显著下降；
4. Unsupported Claim 不得上升超过阈值；
5. 失败则不得默认上线。

---

# 17. 数据闭环与产品壁垒

## 17.1 Moat Stack

从弱到强：

1. Model（弱，可替换）；
2. Prompt；
3. Workflow；
4. IRRS Rule Library；
5. Cross-cultural Issue Taxonomy；
6. Fix Library；
7. Human-reviewed Evidence；
8. Audit History；
9. Benchmark Data；
10. Outcome Data（整改前后真实结果）。

## 17.2 长期数据资产

未来可能形成：

- 不同业态最常见的国际访客摩擦；
- 不同目标访客 Profile 的差异；
- 哪些整改最有效；
- 什么问题高频但低影响；
- 哪些问题最容易导致任务失败。

禁止把“未来可能积累数据”当成当前已存在壁垒。

---

# 18. Metrics

## 18.1 North Star（试点阶段）

**Verified Critical Issues Resolved per Active Project**

为什么不用“报告生成数量”：

因为报告是输出，问题真正解决才是价值。

## 18.2 Activation

- Project Created；
- Evidence Uploaded；
- First Audit Completed；
- First Finding Reviewed。

## 18.3 Value

- Findings Accepted Rate；
- Critical Issues Resolved；
- Retest Pass Rate；
- Time to First Useful Finding；
- 用户认为“此前未意识到”的问题比例。

## 18.4 Business

- Pilot → Paid Conversion；
- 每店/场馆平均整改项目；
- Expansion to Additional Locations；
- Renewal；
- Service Gross Margin。

## 18.5 Guardrail

- False Positive Rate；
- Unsupported Claim Rate；
- Privacy Incident = 0；
- Audit Completion Failure Rate；
- Cost per Audit。

---

# 19. 商业模式假设

> 以下全部是需要验证的假设，不是已验证定价。

## 19.1 单店

- 单次 International Visitor Readiness Audit；
- 整改服务包；
- 持续监测订阅。

## 19.2 连锁 / 商圈 / 文旅

- 按门店数量或项目周期采购；
- 标准模板 + 批量验收 + Dashboard；
- 服务 + SaaS 混合。

## 19.3 最合理的早期商业形态

**Service-led SaaS**：

先人工帮助客户完成验收和整改，记录重复步骤；只有重复且稳定的部分才自动化。

这能避免早期把错误假设写进复杂平台。

---

# 20. 运营设计

## 20.1 Acquisition

最初不要追求大规模投放，优先：

- 比赛合作商户；
- 上海实际门店；
- 老字号 / 文创 / 场馆；
- 商圈运营方；
- 朋友可触达的小店。

## 20.2 Onboarding

目标：10 分钟内完成第一个可分析项目。

不要一开始要求：

- 完整企业认证；
- 20 项资料；
- 复杂组织架构。

## 20.3 AI Ops

每周/每版本：

- 查看失败案例；
- 分类 Issue；
- 加入 Eval Dataset；
- 调整 Rule / Prompt；
- Regression；
- 发布版本记录。

---

# 21. 用户验证计划（比写代码更优先）

## 21.1 Interview 不问什么

不要问：

- “你觉得这个产品好不好？”
- “如果有这个工具你会不会用？”
- “AI 国际化是不是趋势？”

## 21.2 应问什么

围绕最近一次真实事件：

1. 最近一次接待不会中文的客人是什么时候？
2. 他当时要完成什么？
3. 第一步发生了什么？
4. 卡在哪里？
5. 店员怎么解决？
6. 花了多久？
7. 有没有放弃下单/购买？
8. 你后来改过什么？
9. 为此花过钱吗？
10. 如果再发生一次，你准备怎么处理？

## 21.3 48 小时最小验证

优先找 3–5 个真实对象：

- 1 家餐饮；
- 1 家咖啡/零售；
- 1 个文化/展览空间；
- 可选 1–2 名国际访客。

先人工做一次 Audit，观察：

- 商户是否认可 Finding；
- 是否愿意提供更多资料；
- 是否愿意执行整改；
- 是否愿意让我们 Retest；
- 是否问价格；
- 是否愿意介绍另一个商户。

---

# 22. Evidence Gate 与 Kill Criteria

## 22.1 继续条件

### E2

至少 3 个访谈中出现：

- 具体近期事件；
- 明确失败环节；
- 已发生代价。

### E3

至少出现一种行为：

- 商户曾经花时间/钱处理；
- 愿意给资料；
- 愿意让我们现场/远程 Audit；
- 愿意整改并 Retest；
- 主动要求给其他门店使用。

### E4

任一：

- 真实付费 Pilot；
- 定金；
- 正式采购意向/预算；
- 多次续用；
- 转介绍。

## 22.2 Kill Criteria

满足以下任一，应暂停或重构：

1. 5–8 个目标商户都举不出具体国际访客失败事件；
2. 即使有问题，商户认为代价极低且没有整改意愿；
3. 通用 AI + 一张 Prompt 已能完成 80% 且用户不需要历史/流程/复测；
4. Buyer 和 User 无法建立可触达关系；
5. 必须依赖我们当前无法获得的数据/系统权限；
6. 真实 Audit 输出大量 false positive，且无法通过标准化/证据机制改善；
7. 付费方只愿意购买人工咨询，软件部分没有重复价值，则转为服务业务而不是强行 SaaS。

---

# 23. 风险清单

| 风险 | 等级 | 应对 |
|---|---|---|
| 问题存在但不够痛 | 极高 | 用户事件访谈 + Kill Criteria |
| LLM 直接替代 | 高 | 把核心放在标准/证据/流程/复测 |
| Synthetic User 竞品扩张到线下 | 中高 | 线下 Evidence + SMB Workflow + 本地标准 |
| AI 幻觉 | 高 | Evidence grounding + rule engine + human review |
| 上传内容涉及个人信息 | 高 | Data minimization + redaction + retention policy |
| URL 自动访问引入 SSRF | 高 | 网络隔离、allowlist、DNS/IP 校验 |
| 项目过度复杂 | 高 | P0/P1/P2，先单店 vertical slice |
| “评分”被误解为官方认证 | 中高 | 明确 Internal Standard，禁止权威暗示 |
| 早期多模型导致复杂度 | 中 | 单主模型 + provider interface |

---

# 24. Roadmap

## Phase 0：Discovery（现在）

- 3–5 个事件访谈；
- 1 个手工 Audit；
- 验证 E2；
- 决定 Go / Reframe / Stop。

## Phase 1：Competition MVP

- Project；
- Evidence；
- Audit；
- Findings；
- Human Review；
- Retest；
- Before/After Demo。

## Phase 2：Pilot

- 真实商户；
- Guest Layer；
- 任务协作；
- 真实整改；
- 记录结果。

## Phase 3：Multi-location

- 多门店；
- 组织权限；
- Template；
- Benchmark；
- API。

---

# 25. 比赛 Demo Script（90 秒）

1. **问题**：外国游客能翻译文字，但不代表能完成服务流程；
2. 打开一个真实门店项目；
3. 展示上传的门头/菜单/二维码；
4. Run Audit；
5. Journey Map 出现 2–3 个 FAIL；
6. 点击一个 FAIL；
7. 显示 Evidence + Rule + Suggested Fix；
8. 展示整改后的新证据；
9. Retest；
10. FAIL → PASS；
11. Before/After；
12. 结尾：

> **我们不是再做一个外国游客助手。我们帮助城市里的服务本身，准备好接住他们。**

---

# 26. 项目成功定义

比赛阶段的“成功”不是功能数量，而是：

- 至少一个真实可访问 MVP；
- 至少一个端到端 Audit → Fix → Retest；
- Findings 有 Evidence；
- 一个真实商户/场馆案例；
- 至少一个 E2 级用户事件证据；
- 初步证明 ChatGPT/Claude 只能替代部分推理，而不能替代完整工作流。

---

# 27. 参考资料（检索日期：2026-09-07）

1. qrx-joe/option-skill — Evidence-first opportunity decision  
   https://github.com/qrx-joe/option-skill
2. PersonaQA Platform — simulated customers, real browser journeys, evidence-backed findings  
   https://persona.qa/platform/
3. Synthetic Users — AI agents operating real Chromium and returning usability reports  
   https://docs.syntheticusers.io/
4. Applause Localization Testing  
   https://www.applause.com/localization-testing/
5. Lokalise Workflows  
   https://docs.lokalise.com/en/articles/9582608-workflows
6. Shanghai Meet China pilot  
   https://english.shanghai.gov.cn/en-Latest-WhatsNew/20260630/9a1f9e07b5fb434299c9598f506d725e.html
7. Shanghai Hu Xiaoyou service hub  
   https://english.shanghai.gov.cn/en-Latest-WhatsNew/20260821/b80da4f5ed2f4400821e770e98a55e32.html
8. WCAG 2.2  
   https://www.w3.org/TR/WCAG22/
9. OWASP API Security Top 10 2023  
   https://owasp.org/API-Security/

---

# 28. 文档维护规则

- 任何“用户痛点”“市场规模”“愿意付费”结论必须标注证据来源；
- PRD 不得把 Hypothesis 偷换成 Fact；
- 每次用户访谈后更新 `Evidence Log`；
- 每次 Scope 变化记录 Decision Log；
- P0 需求删除必须说明原因；
- AI Rule / Standard 的修改必须版本化；
- 比赛后若证据未达到 E2/E3，不因已经写了代码而继续投入。

