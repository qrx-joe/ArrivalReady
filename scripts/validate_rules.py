#!/usr/bin/env python
"""Validate an IRRS rule-set file against its JSON Schema plus cross-rule invariants.

Contract:
    Input:  one rules YAML file (parsed as JSON before validation) and the
            sibling rule.schema.json for its version directory.
    Output: exit 0 if valid, exit 1 with a list of violations.

Cross-rule invariants enforced here (schema cannot express them):
    - rule codes are unique within the file;
    - the code's dimension segment (IRRS-D3-xxx) matches the rule's dimension;
    - a draft standard may not contain active/retired rules (publication moves
      the whole package, never individual rules in place).

Usage:
    python scripts/validate_rules.py standards/irrs/0.1.0/rules.yaml
    python scripts/validate_rules.py --fixtures standards/irrs/0.1.0/fixtures

The --fixtures mode treats files named valid-*.yaml as must-pass and
invalid-*.yaml as must-fail; it is the offline negative-test entry that CI
(B04) runs without any model key or database.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

import yaml

try:
    from jsonschema import Draft202012Validator
except ImportError:  # pragma: no cover
    print("error: jsonschema is required (pip install jsonschema)", file=sys.stderr)
    sys.exit(2)


def load_rule_set(path: Path) -> tuple[dict, list[str]]:
    """Parse YAML and return (data, errors). YAML syntax errors are reported,
    never raised, so one bad fixture does not abort a batch run."""
    try:
        with path.open(encoding="utf-8") as fh:
            return yaml.safe_load(fh), []
    except yaml.YAMLError as exc:
        return {}, [f"yaml parse error: {exc}"]


def validate(data: dict, schema: dict) -> list[str]:
    errors = [
        f"{e.json_path}: {e.message}"
        for e in Draft202012Validator(schema).iter_errors(data)
    ]

    seen: set[str] = set()
    for i, rule in enumerate(data.get("rules", [])):
        code = rule.get("code", f"<rules[{i}] missing code>")
        if code in seen:
            errors.append(f"rules[{i}]: duplicate rule code {code}")
        seen.add(code)

        declared = rule.get("dimension")
        if declared and not code.startswith(f"IRRS-{declared}-"):
            errors.append(
                f"{code}: code prefix does not match dimension {declared}"
            )

    standard_status = data.get("standard", {}).get("status")
    if standard_status == "draft":
        for i, rule in enumerate(data.get("rules", [])):
            if rule.get("status") != "draft":
                errors.append(
                    f"rules[{i}] ({rule.get('code')}): rule status "
                    f"{rule.get('status')} not allowed while standard is draft"
                )
    return errors


def main() -> int:
    args = sys.argv[1:]
    if not args:
        print(__doc__)
        return 2

    if args[0] == "--fixtures":
        fixtures_dir = Path(args[1])
        failures: list[str] = []
        for path in sorted(fixtures_dir.glob("*.yaml")):
            schema = json.loads(
                (fixtures_dir.parent / "rule.schema.json").read_text(encoding="utf-8")
            )
            data, errors = load_rule_set(path)
            if not errors:
                errors = validate(data, schema)
            name, must_pass = path.name, path.name.startswith("valid-")
            ok = not errors if must_pass else bool(errors)
            print(f"{'PASS' if ok else 'FAIL'}  {name} (expected {'valid' if must_pass else 'invalid'})")
            if not ok:
                failures.append(name)
                for err in errors:
                    print(f"      {err}")
        if failures:
            print(f"fixture failures: {failures}")
            return 1
        print("all fixtures behaved as expected")
        return 0

    target = Path(args[0])
    schema_path = target.parent / "rule.schema.json"
    data, errors = load_rule_set(target)
    if errors:
        for err in errors:
            print(err)
        return 1
    schema = json.loads(schema_path.read_text(encoding="utf-8"))
    errors = validate(data, schema)
    for err in errors:
        print(err)
    print(f"{target}: {'VALID' if not errors else f'{len(errors)} error(s)'}")
    return 1 if errors else 0


if __name__ == "__main__":
    sys.exit(main())
