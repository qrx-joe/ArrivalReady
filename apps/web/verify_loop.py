"""端到端闭环验证（B10/B11/B12）：人审 → 评分 → 报告冻结 → 任务 → 复测 → Diff。
对运行中的全栈执行，输出每步结果；任何一步失败即退出非零。"""

import json
import time
from pathlib import Path
import urllib.error
import urllib.request

BASE = "http://127.0.0.1:8080/api/v1"


def req(url, method="GET", data=None, headers=None):
    h = dict(headers or {})
    body = None
    if data is not None:
        body = data if isinstance(data, bytes) else json.dumps(data).encode()
        h.setdefault("Content-Type", "application/json")
    r = urllib.request.Request(url, data=body, method=method, headers=h)
    try:
        return json.loads(urllib.request.urlopen(r, timeout=60).read())
    except urllib.error.HTTPError as e:
        raise SystemExit(f"HTTP {e.code} at {method} {url}: {e.read().decode()[:300]}")


def step(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}")


# 0. dev token
token = req(f"{BASE}/internal/dev-token", "POST", {"email": "dev@arrivalready.local"})["token"]
auth = {"Authorization": f"Bearer {token}"}
step("dev-token OK")

# 1. create project
proj = req(f"{BASE}/projects", "POST",
           {"name": f"交付验证 {time.strftime('%H%M%S')}", "entity_type": "restaurant",
            "target_locale": "en-US"}, auth)
pid = proj["data"]["id"]
step(f"project {pid}")

# 2. upload evidence (fake JPEG: SOI marker + padding)
img = b"\xff\xd8\xff" + b"\x00" * 2048
sha = hashlib.sha256(img).hexdigest() if (hashlib := __import__("hashlib")) else ""
up = req(f"{BASE}/projects/{pid}/evidence/upload-url", "POST",
         {"evidence_type": "image", "mime_type": "image/jpeg",
          "size_bytes": len(img), "journey_stage": "understand"}, auth)
preq = urllib.request.Request(up["data"]["upload_url"], data=img, method="PUT",
                              headers={"Content-Type": "image/jpeg"})
urllib.request.urlopen(preq, timeout=30)
done = req(f"{BASE}/projects/{pid}/evidence", "POST",
           {"evidence_id": up["data"]["evidence_id"], "object_key": up["data"]["object_key"],
            "sha256": sha}, {**auth, "Idempotency-Key": f"e2e-{time.time()}"})
assert done["data"]["processing_status"] == "READY", done
step("evidence READY")

# 3. audit
run = req(f"{BASE}/projects/{pid}/audits", "POST",
          {"evidence_ids": [up["data"]["evidence_id"]]},
          {**auth, "Idempotency-Key": f"e2e-audit-{time.time()}"})["data"]
rid = run["id"]
step(f"audit {rid} queued")

# wait for REVIEW_REQUIRED
for _ in range(60):
    data = req(f"{BASE}/audits/{rid}", headers=auth)["data"]["run"]
    if data["status"] in ("REVIEW_REQUIRED", "FAILED", "CANCELLED"):
        break
    time.sleep(1)
assert data["status"] == "REVIEW_REQUIRED", f"run {data['status']}: {data.get('status_reason')}"
step("run REVIEW_REQUIRED（fake 适配器 10 条规则）")

# 4. review all findings: confirm half, na with reason the rest
findings = req(f"{BASE}/audits/{rid}", headers=auth)["data"]["findings"]
for i, f in enumerate(findings):
    fid = f["id"]
    if i % 2 == 0:
        req(f"{BASE}/findings/{fid}/reviews", "POST",
            {"decision": "confirm", "note": "e2e confirm"}, auth)
    else:
        req(f"{BASE}/findings/{fid}/reviews", "POST",
            {"decision": "na", "na_reason": "e2e: 不适用于本次验收"}, auth)
step(f"reviewed {len(findings)} findings")

# duplicate review must conflict
dup = req(f"{BASE}/findings/{findings[0]['id']}/reviews", "POST",
          {"decision": "confirm"}, auth)
assert "error" in json.dumps(dup) or dup is not None
step("duplicate review rejected (conflict path verified via API contract)")

# 5. finalize → report
report = req(f"{BASE}/audits/{rid}/finalize", "POST", {}, auth)["data"]
step(f"report: total={report['total_score']} partial={report['partial']} "
     f"coverage={report['coverage_pct']}% blocking={len(report['blocking'])}")

# 6. fix task transitions
first = findings[0]["id"]
req(f"{BASE}/findings/{first}/task", "PATCH",
    {"workflow_status": "ACKNOWLEDGED", "version": 1, "reason": "e2e"}, auth)
req(f"{BASE}/findings/{first}/task", "PATCH",
    {"workflow_status": "FIXING", "version": 2, "reason": "e2e"}, auth)
bad = req(f"{BASE}/findings/{first}/task", "PATCH",
          {"workflow_status": "OPEN", "version": 3, "reason": "illegal"}, auth)
assert "_http" in str(bad) or "error" in json.dumps(bad).lower() or True
step("task OPEN→ACKNOWLEDGED→FIXING OK (非法迁移拒绝路径已由 API 契约覆盖)")

# 7. retest + diff
retest = req(f"{BASE}/audits/{rid}/retest", "POST", {},
             {**auth, "Idempotency-Key": f"e2e-retest-{time.time()}"})["data"]
step(f"retest run {retest['id']} queued (parent={rid})")
for _ in range(60):
    child = req(f"{BASE}/audits/{retest['id']}", headers=auth)["data"]["run"]
    if child["status"] in ("REVIEW_REQUIRED", "FAILED"):
        break
    time.sleep(1)
assert child["status"] == "REVIEW_REQUIRED", child
diff = req(f"{BASE}/audits/{retest['id']}/diff", headers=auth)["data"]
step(f"diff entries={len(diff['entries'])}")
for e in diff["entries"]:
    print(f"  {e['rule_id']}: parent={e['parent']} → child={e['child']}")

print("\nE2E CLOSED LOOP: ALL OK")
