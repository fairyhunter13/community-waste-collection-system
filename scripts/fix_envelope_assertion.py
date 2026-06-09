#!/usr/bin/env python3
"""
Rewrites every 'Response envelope has success field' test block in
api/community-waste.postman_collection.json to assert success:true on
2xx responses and success:false on 4xx+ responses.  Idempotent.
"""
import json
from pathlib import Path

ROOT = Path(__file__).parent.parent
PM_FILE = ROOT / "api/community-waste.postman_collection.json"

OLD_BLOCK = [
    'pm.test("Response envelope has success field", function () {',
    '    const body = pm.response.json();',
    '    pm.expect(body).to.have.property("success");',
    '    pm.expect(body.success).to.be.true;',
    '});',
]

NEW_BLOCK = [
    'pm.test("Response envelope has success field", function () {',
    '    const body = pm.response.json();',
    '    pm.expect(body).to.have.property("success");',
    '    if (pm.response.code >= 200 && pm.response.code < 300) {',
    '        pm.expect(body.success).to.be.true;',
    '    } else {',
    '        pm.expect(body.success).to.be.false;',
    '    }',
    '});',
]


def rewrite_exec(exec_lines: list) -> tuple:
    """Return (rewritten exec list, number of replacements made)."""
    n = len(OLD_BLOCK)
    result = []
    count = 0
    i = 0
    while i < len(exec_lines):
        if exec_lines[i:i + n] == OLD_BLOCK:
            result.extend(NEW_BLOCK)
            count += 1
            i += n
        else:
            result.append(exec_lines[i])
            i += 1
    return result, count


def main() -> None:
    pm = json.loads(PM_FILE.read_text())
    total = 0
    for folder in pm.get("item", []):
        for item in folder.get("item", []):
            for event in item.get("event", []):
                script = event.get("script", {})
                exec_lines = script.get("exec", [])
                new_exec, count = rewrite_exec(exec_lines)
                if count > 0:
                    script["exec"] = new_exec
                    total += count
                    print(f"  Fixed: [{folder['name']}] {item['name']}")
    if total == 0:
        print("Already up to date — no changes needed")
    else:
        PM_FILE.write_text(json.dumps(pm, indent=2, ensure_ascii=False) + "\n")
        print(f"\nFixed {total} assertion block(s) in {PM_FILE.name}")


if __name__ == "__main__":
    main()
