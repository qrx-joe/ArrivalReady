# evals/datasets ｜ 素材与 Golden Set 登记库

> 执行方案 B05/B15 要求：真实素材入库前必须有来源与授权记录；**原始素材不提交本仓库**（.gitignore 已覆盖素材目录）。
> 素材攒取从今天开始（ADVICE A-4）：手机随拍随登记，两周即成完整素材库。

## 登记流程

1. 每拍一批素材，在本目录新建 `cases/<case-id>/manifest.yaml`（模板见下）；
2. 素材文件放 `cases/<case-id>/assets/`（本地，不入库）；仓库只保留 manifest；
3. 首次使用某商户素材前，确认 `authorization` 字段已填（口头授权也要记录时间与方式）；
4. 素材进入系统（上传到环境）前核对 R-08 清单：授权记录 ✓ 脱敏 ✓ 组织访问控制 ✓ 私有存储 ✓。

## manifest.yaml 模板

```yaml
case_id: qr-ordering-cn-only-001     # 稳定 id，与 golden case 一致
captured_at: 2026-09-10              # 拍摄/采集时间
scene: restaurant                    # restaurant / retail / museum / venue / other
source_type: storefront              # storefront / menu / qr_landing / payment_signage / other
assets:
  - filename: menu-front.jpg
    sha256: <采集后填写>
    note: 菜单正面
authorization:
  granted: true                      # 未获授权必须为 false，且不得用于演示/评测
  method: verbal                     # verbal / written
  recorded_at: 2026-09-10
  scope: 比赛演示与内部评测           # 授权范围（明确用途）
pii_review:
  faces_visible: false               # 有可见人脸时先脱敏再入库
  redaction_done: true
notes: >
  用途：IRRS-D4-001 正例 / golden case。来源商户同意演示。
```

## 与 Golden Set 的关系（T-006 / B15）

- 20 个 golden case（8 FAIL / 5 WARN / 5 PASS / 2 证据不足）从本登记库中选取；
- 每个 case 的预期（rule_id / status / severity 范围 / 必须引用的证据 / 禁止断言）按 TECH_SPEC §18 格式在 `evals/datasets/irrs-v0.1/` 建立；
- 未登记授权的素材不得进入 golden set，也不得送任何模型供应商。
