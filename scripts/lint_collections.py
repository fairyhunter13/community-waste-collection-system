#!/usr/bin/env python3
"""
Validates api/community-waste.postman_collection.json and
api/community-waste.insomnia_collection.json against structural and
content requirements.  Used by the CI 'contract' job.

Exit code 0 = all checks pass.  Non-zero = at least one failure.
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
PM_FILE = ROOT / "api/community-waste.postman_collection.json"
INS_FILE = ROOT / "api/community-waste.insomnia_collection.json"

POSTMAN_SCHEMA = "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"

# Documented status codes per primary request (matches enrich_postman_examples.py).
PRIMARY_CODES: dict[str, set[int]] = {
    "Create Household": {201, 400, 413, 500},
    "List Households": {200, 500},
    "Get Household": {200, 404, 500},
    "Delete Household": {204, 404, 500},
    "Create Pickup (organic)": {201, 400, 409, 413, 422, 429, 500},
    "Create Pickup (electronic with safety_check)": {201, 400, 409, 413, 422, 429, 500},
    "List Pickups": {200, 400, 500},
    "List Pickups — filter by status": {200, 400, 500},
    "Schedule Pickup": {200, 400, 404, 409, 422, 500},
    "Complete Pickup": {200, 404, 409, 500},
    "Cancel Pickup": {200, 404, 409, 500},
    "Create Payment": {201, 400, 409, 500},
    "List Payments": {200, 400, 500},
    "List Payments — filter by status": {200, 400, 500},
    "Confirm Payment (multipart proof upload)": {200, 400, 404, 500},
    "Waste Summary": {200, 500},
    "Payment Summary": {200, 500},
    "Household History": {200, 404, 500},
}

EXPECTED_FOLDERS = {"Households", "Waste Pickups", "Payments", "Reports", "Cleanup"}
MIN_REQUESTS = 25


def check_postman(pm: dict) -> list[str]:
    errors: list[str] = []

    # Schema version
    schema = pm.get("info", {}).get("schema", "")
    if schema != POSTMAN_SCHEMA:
        errors.append(f"Postman: wrong schema URL: {schema!r} (expected {POSTMAN_SCHEMA!r})")

    # Folder names
    folders = {f["name"] for f in pm.get("item", [])}
    missing_folders = EXPECTED_FOLDERS - folders
    if missing_folders:
        errors.append(f"Postman: missing folders: {missing_folders}")

    # Request count
    total = sum(len(f.get("item", [])) for f in pm.get("item", []))
    if total < MIN_REQUESTS:
        errors.append(f"Postman: only {total} requests (expected >= {MIN_REQUESTS})")

    # Every primary request has the documented examples
    request_map: dict[str, dict] = {}
    for folder in pm.get("item", []):
        folder_name = folder["name"]
        for item in folder.get("item", []):
            request_map[item["name"]] = {"item": item, "folder": folder_name}

    for req_name, expected_codes in PRIMARY_CODES.items():
        entry = request_map.get(req_name)
        if entry is None:
            errors.append(f"Postman: primary request not found: {req_name!r}")
            continue
        folder_name = entry["folder"]
        item = entry["item"]
        responses = item.get("response", [])
        existing_codes = {r["code"] for r in responses}
        missing = expected_codes - existing_codes
        if missing:
            errors.append(
                f"Postman [{folder_name}] {req_name!r}: missing examples for codes {sorted(missing)}"
            )
        # Each example should have required fields
        for r in responses:
            for field in ("name", "code", "status", "header", "body"):
                if field not in r:
                    errors.append(
                        f"Postman [{folder_name}] {req_name!r}: example {r.get('name', '?')!r} missing field {field!r}"
                    )

    # Delete Household must be in the Cleanup folder (teardown runs last in Newman)
    dh = request_map.get("Delete Household")
    if dh and dh["folder"] != "Cleanup":
        errors.append(f"Postman: 'Delete Household' is in {dh['folder']!r} — should be in 'Cleanup'")

    return errors


def check_insomnia(ins: dict) -> list[str]:
    errors: list[str] = []

    # Export format
    fmt = ins.get("__export_format")
    if fmt != 4:
        errors.append(f"Insomnia: wrong export format: {fmt} (expected 4)")

    resources = ins.get("resources", [])
    types = {}
    for r in resources:
        t = r.get("_type", "?")
        types[t] = types.get(t, 0) + 1

    # Required resource types
    for req_type in ("workspace", "environment", "request_group", "request"):
        if types.get(req_type, 0) == 0:
            errors.append(f"Insomnia: no resources of type {req_type!r}")

    # Requests must have non-empty descriptions
    empty_desc = [
        r["name"]
        for r in resources
        if r.get("_type") == "request" and not r.get("description", "").strip()
    ]
    if empty_desc:
        errors.append(
            f"Insomnia: {len(empty_desc)} request(s) have empty description: {empty_desc[:5]}"
        )

    # Count requests
    req_count = types.get("request", 0)
    if req_count < MIN_REQUESTS:
        errors.append(f"Insomnia: only {req_count} requests (expected >= {MIN_REQUESTS})")

    return errors


def check_parity(pm: dict, ins: dict) -> list[str]:
    errors: list[str] = []

    pm_count = sum(len(f.get("item", [])) for f in pm.get("item", []))
    ins_count = sum(1 for r in ins.get("resources", []) if r.get("_type") == "request")

    if pm_count != ins_count:
        errors.append(
            f"Parity: Postman has {pm_count} requests, Insomnia has {ins_count} — counts must match"
        )

    # Folder-level parity: for each (method, path-suffix) the folder name must match.
    # Build Insomnia folder map: request name → group name
    ins_groups = {r["_id"]: r["name"] for r in ins.get("resources", []) if r["_type"] == "request_group"}
    ins_folder_by_name: dict[str, str] = {}
    for r in ins.get("resources", []):
        if r.get("_type") == "request":
            gname = ins_groups.get(r.get("parentId", ""), "")
            ins_folder_by_name[r["name"]] = gname

    # Build Postman folder map: request name → folder name
    pm_folder_by_name: dict[str, str] = {}
    for folder in pm.get("item", []):
        for item in folder.get("item", []):
            pm_folder_by_name[item["name"]] = folder["name"]

    for req_name in set(pm_folder_by_name) | set(ins_folder_by_name):
        pm_folder = pm_folder_by_name.get(req_name)
        ins_folder = ins_folder_by_name.get(req_name)
        if pm_folder is None:
            errors.append(f"Parity: {req_name!r} in Insomnia ({ins_folder}) but not in Postman")
        elif ins_folder is None:
            errors.append(f"Parity: {req_name!r} in Postman ({pm_folder}) but not in Insomnia")
        elif pm_folder != ins_folder:
            errors.append(
                f"Parity: {req_name!r} folder mismatch — Postman={pm_folder!r}, Insomnia={ins_folder!r}"
            )

    return errors


def main() -> int:
    try:
        pm = json.loads(PM_FILE.read_text())
    except Exception as exc:
        print(f"FAIL: could not parse Postman collection: {exc}", file=sys.stderr)
        return 1

    try:
        ins = json.loads(INS_FILE.read_text())
    except Exception as exc:
        print(f"FAIL: could not parse Insomnia collection: {exc}", file=sys.stderr)
        return 1

    all_errors: list[str] = []
    all_errors += check_postman(pm)
    all_errors += check_insomnia(ins)
    all_errors += check_parity(pm, ins)

    if all_errors:
        for e in all_errors:
            print(f"FAIL: {e}", file=sys.stderr)
        return 1

    print("OK: postman + insomnia collections valid and in sync")
    return 0


if __name__ == "__main__":
    sys.exit(main())
