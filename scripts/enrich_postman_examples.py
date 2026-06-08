#!/usr/bin/env python3
"""
Enrich api/community-waste.postman_collection.json so that every
primary request carries a saved response example for each status code
documented in api/openapi.yaml, and moves Delete Household to the
Households folder.

Run: python3 scripts/enrich_postman_examples.py
"""
import copy
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
PM_FILE = ROOT / "api/community-waste.postman_collection.json"
OUT_FILE = PM_FILE

# ── Status code → example body (from api/openapi.yaml examples) ──────────────

HEADER_JSON = [
    {"key": "Content-Type", "value": "application/json"},
    {"key": "X-Request-Id", "value": "7a3f1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c"},
]

META_EXAMPLE = {
    "request_id": "7a3f1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c",
    "trace_id": "0000000000000000deadbeefcafebabe",
    "span_id": "0102030405060708",
}


def _err(code: str, message: str) -> dict:
    return {
        "success": False,
        "error": {"code": code, "message": message},
        "meta": META_EXAMPLE,
    }


STATUS_BODIES: dict[int, tuple[str, dict]] = {
    200: ("OK", None),
    201: ("Created", None),
    204: ("No Content", None),
    400: ("Bad Request", _err("VALIDATION_ERROR", "owner_name is required")),
    404: ("Not Found", _err("NOT_FOUND", "resource not found")),
    409: ("Conflict", _err("CONFLICT", "business state conflict")),
    413: ("Request Entity Too Large", _err("REQUEST_TOO_LARGE", "request body exceeds size limit")),
    422: ("Unprocessable Entity", _err("BUSINESS_RULE_VIOLATION", "electronic pickup requires safety_check to be true before scheduling")),
    429: ("Too Many Requests", _err("RATE_LIMITED", "too many requests")),
    500: ("Internal Server Error", _err("INTERNAL_ERROR", "internal server error")),
}

# ── Per-request documented status codes (from openapi.yaml) ──────────────────
#
# Key = exact request name in the collection (primary requests only).
# Value = set of int status codes that endpoint documents.
# Sibling "— 400 / 404 / etc." requests are intentionally absent from this
# table; they stay untouched.

PRIMARY_CODES: dict[str, set[int]] = {
    "Create Household": {201, 400, 413, 500},
    "List Households": {200, 500},
    "Get Household": {200, 404, 500},
    "Delete Household": {204, 404, 500},
    # Both organic and electronic variants represent the createPickup endpoint.
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

# Custom bodies for specific endpoints (overrides the generic STATUS_BODIES).
# Only needed when the generic body would be misleading.
ENDPOINT_OVERRIDES: dict[tuple[str, int], dict] = {
    ("Delete Household", 204): None,  # no body for 204
    ("Create Household", 201): {
        "success": True,
        "data": {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "owner_name": "Budi Santoso",
            "address": "Jl. Merdeka No. 5, Jakarta",
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T09:32:00Z",
        },
    },
    ("Create Pickup (organic)", 201): {
        "success": True,
        "data": {
            "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "type": "organic",
            "status": "pending",
            "pickup_date": None,
            "safety_check": False,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T09:32:00Z",
        },
    },
    ("Create Pickup (electronic with safety_check)", 201): {
        "success": True,
        "data": {
            "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "type": "electronic",
            "status": "pending",
            "pickup_date": None,
            "safety_check": True,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T09:32:00Z",
        },
    },
    ("Schedule Pickup", 200): {
        "success": True,
        "data": {
            "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "type": "plastic",
            "status": "scheduled",
            "pickup_date": "2026-01-22T11:15:00Z",
            "safety_check": False,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T10:15:00Z",
        },
    },
    ("Complete Pickup", 200): {
        "success": True,
        "data": {
            "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "type": "plastic",
            "status": "completed",
            "pickup_date": "2026-01-22T11:15:00Z",
            "safety_check": False,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2026-01-22T11:15:00Z",
        },
    },
    ("Cancel Pickup", 200): {
        "success": True,
        "data": {
            "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "type": "organic",
            "status": "canceled",
            "pickup_date": None,
            "safety_check": False,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T11:00:00Z",
        },
    },
    ("Create Payment", 201): {
        "success": True,
        "data": {
            "id": "880e8400-e29b-41d4-a716-446655440002",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "waste_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "amount": "50000.00",
            "status": "pending",
            "proof_file_url": None,
            "payment_date": None,
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T09:32:00Z",
        },
    },
    ("Confirm Payment (multipart proof upload)", 200): {
        "success": True,
        "data": {
            "id": "880e8400-e29b-41d4-a716-446655440002",
            "household_id": "550e8400-e29b-41d4-a716-446655440000",
            "waste_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
            "amount": "50000.00",
            "status": "paid",
            "proof_file_url": "http://localhost:9000/waste-proofs/proof.jpg",
            "payment_date": "2025-11-15T10:15:00Z",
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T10:15:00Z",
        },
    },
    ("Get Household", 404): _err("NOT_FOUND", "household not found"),
    ("Delete Household", 404): _err("NOT_FOUND", "household not found"),
    ("Schedule Pickup", 404): _err("NOT_FOUND", "pickup not found"),
    ("Complete Pickup", 404): _err("NOT_FOUND", "pickup not found"),
    ("Cancel Pickup", 404): _err("NOT_FOUND", "pickup not found"),
    ("Create Pickup (organic)", 409): _err("CONFLICT", "household has a pending payment"),
    ("Create Pickup (electronic with safety_check)", 409): _err("CONFLICT", "household has a pending payment"),
    ("Schedule Pickup", 409): _err("CONFLICT", "pickup is not in pending status"),
    ("Complete Pickup", 409): _err("CONFLICT", "pickup is not in scheduled status"),
    ("Cancel Pickup", 409): _err("CONFLICT", "pickup cannot be canceled in its current state"),
    ("Create Payment", 409): _err("CONFLICT", "payment already exists for this pickup"),
    ("Confirm Payment (multipart proof upload)", 404): _err("NOT_FOUND", "payment not found"),
    ("Household History", 404): _err("NOT_FOUND", "household not found"),
    ("Waste Summary", 200): {
        "success": True,
        "data": {
            "by_type": [
                {"type": "plastic", "total": 7, "by_status": {"pending": 2, "completed": 5}},
                {"type": "organic", "total": 3, "by_status": {"pending": 2, "canceled": 1}},
            ]
        },
    },
    ("Payment Summary", 200): {
        "success": True,
        "data": {
            "by_status": [
                {"status": "pending", "count": 3, "total_amount": "150000.00"},
                {"status": "paid", "count": 12, "total_amount": "600000.00"},
                {"status": "failed", "count": 1, "total_amount": "50000.00"},
            ],
            "total_revenue": "600000.00",
        },
    },
    ("Household History", 200): {
        "success": True,
        "data": {
            "household": {
                "id": "550e8400-e29b-41d4-a716-446655440000",
                "owner_name": "Budi Santoso",
                "address": "Jl. Merdeka No. 5, Jakarta",
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            },
            "pickups": [
                {
                    "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                    "type": "plastic",
                    "status": "completed",
                    "pickup_date": "2026-01-22T11:15:00Z",
                }
            ],
            "payments": [
                {"id": "880e8400-e29b-41d4-a716-446655440002", "amount": "50000.00", "status": "paid"}
            ],
        },
    },
    ("List Households", 200): {
        "success": True,
        "data": [
            {
                "id": "550e8400-e29b-41d4-a716-446655440000",
                "owner_name": "Budi Santoso",
                "address": "Jl. Merdeka No. 5, Jakarta",
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            }
        ],
        "meta": {"page": 1, "per_page": 20, "total": 1, "total_pages": 1},
    },
    ("Get Household", 200): {
        "success": True,
        "data": {
            "id": "550e8400-e29b-41d4-a716-446655440000",
            "owner_name": "Budi Santoso",
            "address": "Jl. Merdeka No. 5, Jakarta",
            "created_at": "2025-11-15T09:32:00Z",
            "updated_at": "2025-11-15T09:32:00Z",
        },
    },
    ("List Pickups", 200): {
        "success": True,
        "data": [
            {
                "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                "household_id": "550e8400-e29b-41d4-a716-446655440000",
                "type": "plastic",
                "status": "pending",
                "pickup_date": None,
                "safety_check": False,
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            }
        ],
        "meta": {"page": 1, "per_page": 20, "total": 1, "total_pages": 1},
    },
    ("List Pickups — filter by status", 200): {
        "success": True,
        "data": [
            {
                "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                "household_id": "550e8400-e29b-41d4-a716-446655440000",
                "type": "plastic",
                "status": "pending",
                "pickup_date": None,
                "safety_check": False,
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            }
        ],
        "meta": {"page": 1, "per_page": 20, "total": 1, "total_pages": 1},
    },
    ("List Payments", 200): {
        "success": True,
        "data": [
            {
                "id": "880e8400-e29b-41d4-a716-446655440002",
                "household_id": "550e8400-e29b-41d4-a716-446655440000",
                "waste_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                "amount": "50000.00",
                "status": "pending",
                "proof_file_url": None,
                "payment_date": None,
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            }
        ],
        "meta": {"page": 1, "per_page": 20, "total": 1, "total_pages": 1},
    },
    ("List Payments — filter by status", 200): {
        "success": True,
        "data": [
            {
                "id": "880e8400-e29b-41d4-a716-446655440002",
                "household_id": "550e8400-e29b-41d4-a716-446655440000",
                "waste_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
                "amount": "50000.00",
                "status": "paid",
                "proof_file_url": "http://localhost:9000/waste-proofs/proof.jpg",
                "payment_date": "2025-11-15T10:15:00Z",
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T10:15:00Z",
            }
        ],
        "meta": {"page": 1, "per_page": 20, "total": 1, "total_pages": 1},
    },
}


def _make_example(name: str, code: int, original_request: dict) -> dict:
    status_text, generic_body = STATUS_BODIES[code]
    body_data = ENDPOINT_OVERRIDES.get((name, code), generic_body)
    headers = [] if code == 204 else list(HEADER_JSON)
    body_str = "" if (code == 204 or body_data is None) else json.dumps(body_data, indent=2)
    return {
        "name": f"{code} {status_text}",
        "originalRequest": copy.deepcopy(original_request),
        "status": status_text,
        "code": code,
        "_postman_previewlanguage": "json" if body_str else "text",
        "header": headers,
        "cookie": [],
        "body": body_str,
    }


def enrich(pm: dict) -> dict:
    # Build lookup: request name → folder index
    folder_by_request: dict[str, int] = {}
    for fi, folder in enumerate(pm["item"]):
        for item in folder.get("item", []):
            folder_by_request[item["name"]] = fi

    # ── Step 1: enrich primary requests with missing status-code examples ──
    for folder in pm["item"]:
        for item in folder.get("item", []):
            req_name = item["name"]
            if req_name not in PRIMARY_CODES:
                continue
            documented_codes = PRIMARY_CODES[req_name]
            existing_codes = {r["code"] for r in item.get("response", [])}
            missing = sorted(documented_codes - existing_codes)
            orig_req = item["request"]
            for code in missing:
                item.setdefault("response", []).append(
                    _make_example(req_name, code, orig_req)
                )

    # ── Step 2: move Delete Household to the Households folder ──────────────
    reports_folder = next(f for f in pm["item"] if f["name"] == "Reports")
    households_folder = next(f for f in pm["item"] if f["name"] == "Households")

    del_idx = next(
        (i for i, it in enumerate(reports_folder["item"]) if it["name"] == "Delete Household"),
        None,
    )
    if del_idx is not None:
        dh_item = reports_folder["item"].pop(del_idx)
        households_folder["item"].append(dh_item)
        print("Moved 'Delete Household' from Reports → Households")
    else:
        print("'Delete Household' already in correct folder — skipping move")

    return pm


def main() -> None:
    pm = json.loads(PM_FILE.read_text())
    pm = enrich(pm)
    PM_FILE.write_text(json.dumps(pm, indent=2, ensure_ascii=False) + "\n")
    # Summary
    for folder in pm["item"]:
        for item in folder.get("item", []):
            if item["name"] in PRIMARY_CODES:
                codes = sorted(r["code"] for r in item.get("response", []))
                print(f"  [{folder['name']}] {item['name']}: examples {codes}")
    print(f"\nWrote {PM_FILE}")


if __name__ == "__main__":
    main()
    sys.exit(0)
