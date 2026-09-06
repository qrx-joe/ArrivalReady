#!/usr/bin/env python
"""Validate Arrival Ready contracts: JSON-Schema fixtures and the OpenAPI file.

Contract:
    Input:  contracts/json-schema/*.schema.json, contracts/fixtures/*.json,
            contracts/openapi/arrivalready.yaml
    Output: exit 0 when everything holds; exit 1 with per-file violations.

Fixture naming convention (mirrors scripts/validate_rules.py):
    <schemaPrefix>.valid*.json    -> must validate against <schemaPrefix>.schema.json
    <schemaPrefix>.invalid-*.json -> must be REJECTED by that schema

Cross-schema $refs (job_payload -> normalized_evidence) are resolved through a
local registry keyed by each schema's $id, so validation works fully offline.
The same fixtures are the shared truth for Go and Python consumers (B03 verify);
the Go side consumes them from B04's CI onward.

OpenAPI checks: YAML parse, OpenAPI version, and every internal $ref
("#/...") resolving inside the document. If `openapi-spec-validator` is
installed it runs as an additional full lint; otherwise the structural checks
stand alone (reported as such, never as a full lint).

Usage:
    python scripts/validate_contracts.py --fixtures contracts/fixtures
    python scripts/validate_contracts.py --openapi contracts/openapi/arrivalready.yaml
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import yaml

from jsonschema import Draft202012Validator
from referencing import Registry, Resource


def build_registry(schema_dir: Path) -> Registry:
    registry = Registry()
    for path in sorted(schema_dir.glob("*.schema.json")):
        schema = json.loads(path.read_text(encoding="utf-8"))
        # $schema key is present in every contract schema, so from_contents
        # auto-detects the draft specification.
        resource = Resource.from_contents(schema)
        registry = registry.with_resource(schema["$id"], resource)
    return registry


def check_fixtures(fixtures_dir: Path) -> bool:
    schema_dir = fixtures_dir.parent / "json-schema"
    registry = build_registry(schema_dir)
    ok = True
    for path in sorted(fixtures_dir.glob("*.json")):
        prefix = path.name.split(".")[0]
        schema_path = schema_dir / f"{prefix}.schema.json"
        if not schema_path.exists():
            print(f"FAIL  {path.name}: no schema {schema_path.name}")
            ok = False
            continue
        schema = json.loads(schema_path.read_text(encoding="utf-8"))
        instance = json.loads(path.read_text(encoding="utf-8"))
        errors = [
            f"{e.json_path}: {e.message}"
            for e in Draft202012Validator(
                schema, registry=registry
            ).iter_errors(instance)
        ]
        must_pass = ".valid" in path.name  # matches .valid.json / .valid-*.json
        passed = not errors if must_pass else bool(errors)
        expectation = "valid" if must_pass else "invalid"
        print(f"{'PASS' if passed else 'FAIL'}  {path.name} (expected {expectation})")
        if not passed:
            ok = False
            for err in errors:
                print(f"      {err}")
    return ok


def _ref_exists(doc: dict, ref: str) -> bool:
    if not ref.startswith("#/"):
        return True  # external refs are checked against the registry elsewhere
    node = doc
    for part in ref[2:].split("/"):
        part = part.replace("~1", "/").replace("~0", "~")
        if isinstance(node, dict) and part in node:
            node = node[part]
        else:
            return False
    return True


def check_openapi(openapi_path: Path) -> bool:
    doc = yaml.safe_load(openapi_path.read_text(encoding="utf-8"))
    ok = True
    version = str(doc.get("openapi", ""))
    if not version.startswith("3.1"):
        print(f"FAIL  openapi version {version!r} is not 3.1.x")
        ok = False

    def walk(node):
        if isinstance(node, dict):
            for key, value in node.items():
                if key == "$ref" and isinstance(value, str):
                    yield value
                else:
                    yield from walk(value)
        elif isinstance(node, list):
            for item in node:
                yield from walk(item)

    for ref in walk(doc):
        if not _ref_exists(doc, ref):
            print(f"FAIL  unresolved $ref: {ref}")
            ok = False

    paths = doc.get("paths", {})
    if len(paths) < 13:
        print(f"FAIL  expected the full P0 surface (>=13 paths, TECH_SPEC §9), found {len(paths)}")
        ok = False

    try:
        from openapi_spec_validator import validate as oas_validate

        oas_validate(doc)
        print("openapi-spec-validator: full lint passed")
    except ImportError:
        print(
            "note: openapi-spec-validator not installed; "
            "structural + $ref checks ran (not a full lint)"
        )
    except Exception as exc:  # noqa: BLE001 - report any lint failure
        print(f"FAIL  openapi-spec-validator: {exc}")
        ok = False

    print(f"{openapi_path}: {'VALID' if ok else 'INVALID'}")
    return ok


def main() -> int:
    args = sys.argv[1:]
    if len(args) != 2 or args[0] not in ("--fixtures", "--openapi"):
        print(__doc__)
        return 2
    target = Path(args[1])
    if args[0] == "--fixtures":
        return 0 if check_fixtures(target) else 1
    return 0 if check_openapi(target) else 1


if __name__ == "__main__":
    sys.exit(main())
