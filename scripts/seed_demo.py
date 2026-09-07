# 演示种子数据：创建真实店铺项目并经正式证据链路上传图片材料。
# 图片在运行时从网络下载（素材不入库，docs/03 §4），只提交本脚本。
# 幂等：按项目名去重，重复执行会跳过已存在的店铺。
#
# 用法（API 需已启动）：
#   python scripts/seed_demo.py
# 可选环境变量：ARRIVAL_API_BASE（默认 http://localhost:8080/api/v1）、
#   ARRIVAL_DEV_EMAIL（默认 demo@local.dev）

import hashlib
import json
import os
import sys
import time
import urllib.error
import urllib.request
import uuid

API_BASE = os.environ.get("ARRIVAL_API_BASE", "http://localhost:8080/api/v1")
DEV_EMAIL = os.environ.get("ARRIVAL_DEV_EMAIL", "demo@local.dev")
MAX_IMAGE_BYTES = 10 << 20  # 与 services/api evidence service 上限一致

# 图片内容均经人工核对（2026-09-07 搜索），来源标注仅作元数据。
SHOPS = [
    {
        "name": "全聚德·前门店",
        "entity_type": "restaurant",
        "images": [
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/eb84d1fd134b.jpg",
                "stage": "discover",
                "note": "明档切烤鸭（新京报）",
            },
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/e72745c4d39e.jpg",
                "stage": "decide",
                "note": "烤鸭菜单页，含套/半套价格",
            },
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/4b4b8c292d0b.jpg",
                "stage": "remember",
                "note": "盛世牡丹烤鸭宴席（北京日报）",
            },
        ],
    },
    {
        "name": "中国国家博物馆",
        "entity_type": "museum",
        "images": [
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/8243ac456dd1.jpg",
                "stage": "discover",
                "note": "展厅内景（Tripadvisor）",
            },
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/3e9da7c0a537.jpeg",
                "stage": "understand",
                "note": "列宾艺术特展双语展墙",
            },
        ],
    },
    {
        "name": "前门中轴线文创（云居胡同1号）",
        "entity_type": "retail",
        "images": [
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/64fc22eca7ca.jpeg",
                "stage": "discover",
                "note": "门头夜景招牌·集章打卡（首都文明网）",
            },
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/38df9400d75d.jpeg",
                "stage": "decide",
                "note": "非遗文创展台含价签",
            },
            {
                "url": "https://z-cdn.chatglm.cn/image-search-mcp/images-ppt/04c2dba55cf0.jpg",
                "stage": "act",
                "note": "店内老北京冰箱贴陈列（北京日报）",
            },
        ],
    },
]


def http_json(method, path, token=None, idempotency_key=None, payload=None):
    req = urllib.request.Request(f"{API_BASE}{path}", method=method)
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", f"Bearer {token}")
    if idempotency_key:
        req.add_header("Idempotency-Key", idempotency_key)
    data = json.dumps(payload).encode() if payload is not None else None
    with urllib.request.urlopen(req, data=data, timeout=15) as resp:
        return json.loads(resp.read().decode())


def sniff_mime(data):
    if data.startswith(b"\xff\xd8\xff"):
        return "image/jpeg"
    if data.startswith(b"\x89PNG"):
        return "image/png"
    if data[:4] == b"RIFF" and data[8:12] == b"WEBP":
        return "image/webp"
    return None


def download(url):
    req = urllib.request.Request(url, method="GET")
    req.add_header("User-Agent", "Mozilla/5.0 (arrivalready-seed)")
    with urllib.request.urlopen(req, timeout=30) as resp:
        data = resp.read()
    if len(data) > MAX_IMAGE_BYTES:
        raise ValueError(f"超过 10MiB 上限（{len(data)} bytes）")
    mime = sniff_mime(data)
    if not mime:
        raise ValueError("magic bytes 不是 jpeg/png/webp")
    return data, mime


def upload_evidence(token, project_id, img):
    data, mime = download(img["url"])
    up = http_json(
        "POST",
        f"/projects/{project_id}/evidence/upload-url",
        token=token,
        payload={
            "evidence_type": "image",
            "mime_type": mime,
            "size_bytes": len(data),
            "journey_stage": img["stage"],
            "source_uri_note": img["note"],
        },
    )["data"]
    put_req = urllib.request.Request(up["upload_url"], method="PUT", data=data)
    put_req.add_header("Content-Type", mime)
    with urllib.request.urlopen(put_req, timeout=30) as resp:
        if resp.status not in (200, 201):
            raise ValueError(f"PUT 状态 {resp.status}")
    done = http_json(
        "POST",
        f"/projects/{project_id}/evidence",
        token=token,
        idempotency_key=str(uuid.uuid4()),
        payload={
            "evidence_id": up["evidence_id"],
            "object_key": up["object_key"],
            "size_bytes": len(data),
            "sha256": hashlib.sha256(data).hexdigest(),
        },
    )["data"]
    return done.get("processing_status", "?"), done.get("id", "")


def main():
    token = http_json(
        "POST", "/internal/dev-token", payload={"email": DEV_EMAIL}
    )["token"]
    existing = {
        p["name"] for p in http_json("GET", "/projects", token=token)["data"]
    }
    failures = 0
    for shop in SHOPS:
        if shop["name"] in existing:
            print(f"跳过（已存在）：{shop['name']}")
            continue
        project = http_json(
            "POST",
            "/projects",
            token=token,
            payload={
                "name": shop["name"],
                "entity_type": shop["entity_type"],
                "target_locale": "en-US",
            },
        )["data"]
        print(f"已创建：{shop['name']}  id={project['id'][:8]}")
        for img in shop["images"]:
            try:
                status, eid = upload_evidence(token, project["id"], img)
                print(f"  [{status}] {img['note']}  ({img['stage']})")
                if status != "READY":
                    failures += 1
            except (urllib.error.URLError, ValueError, KeyError) as exc:
                failures += 1
                print(f"  [失败] {img['note']}: {exc}")
            time.sleep(0.3)
    if failures:
        print(f"完成，但有 {failures} 份材料未就绪")
        return 1
    print("全部店铺与材料就绪")
    return 0


if __name__ == "__main__":
    sys.exit(main())
