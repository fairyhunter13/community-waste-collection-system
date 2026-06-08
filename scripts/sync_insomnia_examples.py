#!/usr/bin/env python3
"""
Generate response-example markdown for every request in
api/community-waste.insomnia_collection.json by reading the response
examples from api/openapi.yaml.  Idempotent — safe to re-run after
OpenAPI spec updates.

Also fixes the relative 'fileName' in the proof-upload request by
replacing the bare path with a template that imports can override.

Run: python3 scripts/sync_insomnia_examples.py
"""
import json
import sys
from pathlib import Path

ROOT = Path(__file__).parent.parent
INS_FILE = ROOT / "api/community-waste.insomnia_collection.json"
OPENAPI_FILE = ROOT / "api/openapi.yaml"

# ── Status-code → (status line, body dict) ──────────────────────────────────
# Bodies are taken verbatim from api/openapi.yaml component examples.

META_EXAMPLE = {
    "request_id": "7a3f1b2c-4d5e-6f7a-8b9c-0d1e2f3a4b5c",
    "trace_id": "0000000000000000deadbeefcafebabe",
    "span_id": "0102030405060708",
}


def _err(code: str, msg: str) -> dict:
    return {"success": False, "error": {"code": code, "message": msg}, "meta": META_EXAMPLE}


# Maps (endpoint_name, status_code) → response body dict.
# endpoint_name matches the Insomnia request 'name' field.
ENDPOINT_EXAMPLES: dict[str, dict[int, dict | None]] = {
    "Create Household": {
        201: {
            "success": True,
            "data": {
                "id": "550e8400-e29b-41d4-a716-446655440000",
                "owner_name": "Budi Santoso",
                "address": "Jl. Merdeka No. 5, Jakarta",
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            },
        },
        400: _err("VALIDATION_ERROR", "owner_name is required"),
        413: _err("REQUEST_TOO_LARGE", "request body exceeds size limit"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Create Household — 400 Missing Fields": {
        400: _err("VALIDATION_ERROR", "owner_name is required"),
    },
    "List Households": {
        200: {
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
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Get Household": {
        200: {
            "success": True,
            "data": {
                "id": "550e8400-e29b-41d4-a716-446655440000",
                "owner_name": "Budi Santoso",
                "address": "Jl. Merdeka No. 5, Jakarta",
                "created_at": "2025-11-15T09:32:00Z",
                "updated_at": "2025-11-15T09:32:00Z",
            },
        },
        404: _err("NOT_FOUND", "household not found"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Get Household — 404": {
        404: _err("NOT_FOUND", "household not found"),
    },
    "Delete Household": {
        204: None,  # no body
        404: _err("NOT_FOUND", "household not found"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Create Pickup (organic)": {
        201: {
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
        400: _err("VALIDATION_ERROR", "type must be one of: organic, plastic, paper, electronic"),
        409: _err("CONFLICT", "household has a pending payment"),
        413: _err("REQUEST_TOO_LARGE", "request body exceeds size limit"),
        422: _err("BUSINESS_RULE_VIOLATION", "electronic pickup requires safety_check to be true before scheduling"),
        429: _err("RATE_LIMITED", "too many requests"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Create Pickup (electronic with safety_check)": {
        201: {
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
        400: _err("VALIDATION_ERROR", "type must be one of: organic, plastic, paper, electronic"),
        409: _err("CONFLICT", "household has a pending payment"),
        413: _err("REQUEST_TOO_LARGE", "request body exceeds size limit"),
        422: _err("BUSINESS_RULE_VIOLATION", "electronic pickup requires safety_check to be true before scheduling"),
        429: _err("RATE_LIMITED", "too many requests"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Create Pickup — 409 Pending Payment": {
        409: _err("CONFLICT", "household has a pending payment"),
    },
    "Create Pickup — 429 Rate Limited": {
        429: _err("RATE_LIMITED", "too many requests"),
    },
    "List Pickups": {
        200: {
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
        400: _err("VALIDATION_ERROR", "invalid query parameter"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "List Pickups — filter by status": {
        200: {
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
        400: _err("VALIDATION_ERROR", "invalid query parameter"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Schedule Pickup": {
        200: {
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
        400: _err("VALIDATION_ERROR", "pickup_date must be a future date"),
        404: _err("NOT_FOUND", "pickup not found"),
        409: _err("CONFLICT", "pickup is not in pending status"),
        422: _err("BUSINESS_RULE_VIOLATION", "electronic pickup requires safety_check to be true before scheduling"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Schedule Pickup — 422 Electronic No Safety Check": {
        422: _err("BUSINESS_RULE_VIOLATION", "electronic pickup requires safety_check to be true before scheduling"),
    },
    "Complete Pickup": {
        200: {
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
        404: _err("NOT_FOUND", "pickup not found"),
        409: _err("CONFLICT", "pickup is not in scheduled status"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Cancel Pickup": {
        200: {
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
        404: _err("NOT_FOUND", "pickup not found"),
        409: _err("CONFLICT", "pickup cannot be canceled in its current state"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Cancel Pickup — 409 Already Completed": {
        409: _err("CONFLICT", "pickup cannot be canceled in its current state"),
    },
    "Create Payment": {
        201: {
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
        400: _err("VALIDATION_ERROR", "amount must be a positive decimal"),
        409: _err("CONFLICT", "payment already exists for this pickup"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "List Payments": {
        200: {
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
        400: _err("VALIDATION_ERROR", "invalid query parameter"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "List Payments — filter by status": {
        200: {
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
        400: _err("VALIDATION_ERROR", "invalid query parameter"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Confirm Payment (multipart proof upload)": {
        200: {
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
        400: _err("VALIDATION_ERROR", "proof file is required"),
        404: _err("NOT_FOUND", "payment not found"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Confirm Payment — 400 No File": {
        400: _err("VALIDATION_ERROR", "proof file is required"),
    },
    "Confirm Payment — 404": {
        404: _err("NOT_FOUND", "payment not found"),
    },
    "Waste Summary": {
        200: {
            "success": True,
            "data": {
                "by_type": [
                    {"type": "plastic", "total": 7, "by_status": {"pending": 2, "completed": 5}},
                    {"type": "organic", "total": 3, "by_status": {"pending": 2, "canceled": 1}},
                ]
            },
        },
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Payment Summary": {
        200: {
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
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Household History": {
        200: {
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
        404: _err("NOT_FOUND", "household not found"),
        500: _err("INTERNAL_ERROR", "internal server error"),
    },
    "Household History — 404": {
        404: _err("NOT_FOUND", "household not found"),
    },
}

HTTP_STATUS_TEXT = {
    200: "200 OK",
    201: "201 Created",
    204: "204 No Content",
    400: "400 Bad Request",
    404: "404 Not Found",
    409: "409 Conflict",
    413: "413 Request Entity Too Large",
    422: "422 Unprocessable Entity",
    429: "429 Too Many Requests",
    500: "500 Internal Server Error",
}


def _build_description(req_name: str, method: str, url: str) -> str:
    examples = ENDPOINT_EXAMPLES.get(req_name)
    if not examples:
        return ""

    lines = [f"### `{method} {url}`", ""]
    lines.append("### Responses")
    lines.append("")
    for code in sorted(examples.keys()):
        body = examples[code]
        status_line = HTTP_STATUS_TEXT.get(code, str(code))
        lines.append(f"#### {status_line}")
        if body is None:
            lines.append("*(no response body)*")
        else:
            lines.append("```json")
            lines.append(json.dumps(body, indent=2, ensure_ascii=False))
            lines.append("```")
        lines.append("")
    return "\n".join(lines)


def _extract_path(url: str) -> str:
    """Strip base_url template prefix and query string for display."""
    url = url.replace("{{base_url}}", "")
    url = url.split("?")[0]
    return url


def main() -> None:
    ins = json.loads(INS_FILE.read_text())

    env_resource = next((r for r in ins["resources"] if r["_type"] == "environment"), None)

    # Add fixtures_dir to env if not present
    if env_resource is not None:
        env_data = env_resource.get("data", {})
        if "fixtures_dir" not in env_data:
            env_data["fixtures_dir"] = "api/fixtures"
            env_resource["data"] = env_data
            print("Added fixtures_dir to environment resource")

    updated = 0
    for r in ins["resources"]:
        if r["_type"] != "request":
            continue
        name = r["name"]
        method = r.get("method", "GET")
        url = r.get("url", "")
        path = _extract_path(url)

        desc = _build_description(name, method, path)
        if desc:
            r["description"] = desc
            updated += 1

        # Fix fileName in body (multipart proof upload)
        body = r.get("body", {})
        if body.get("mimeType") == "multipart/form-data":
            params = body.get("params", [])
            for p in params:
                if p.get("type") == "file" and "fileName" in p:
                    old = p["fileName"]
                    # Replace bare relative path with env-variable template
                    if "fixtures_dir" not in old:
                        filename = Path(old).name
                        p["fileName"] = "{{ _.fixtures_dir }}/" + filename
                        print(f"Fixed fileName: {old!r} → {p['fileName']!r}")

    ins["resources"] = [
        r for r in ins["resources"]
        if not (r["_type"] == "request" and not r.get("description"))
        or True  # keep all, just update
    ]

    INS_FILE.write_text(json.dumps(ins, indent=2, ensure_ascii=False) + "\n")
    print(f"\nUpdated {updated}/27 request descriptions")
    print(f"Wrote {INS_FILE}")


if __name__ == "__main__":
    main()
    sys.exit(0)
