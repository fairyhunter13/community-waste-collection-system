#!/usr/bin/env bash
# Verify that both API client collections are structurally valid,
# load cleanly in their respective CLI tools, and (when BASE_URL is
# set) all Postman requests pass against the live API.
#
# Usage (static only):
#   bash scripts/verify-collections.sh
#
# Usage (static + live):
#   BASE_URL=http://localhost:8080 bash scripts/verify-collections.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PM_FILE="$ROOT/api/community-waste.postman_collection.json"
INS_FILE="$ROOT/api/community-waste.insomnia_collection.json"

echo "=== Layer A: static lint ==="
python3 "$ROOT/scripts/lint_collections.py"

echo ""
echo "=== Layer B: JSON well-formedness ==="
python3 -c "import json; json.load(open('$PM_FILE'))"
echo "Postman: valid JSON"
python3 -c "import json; json.load(open('$INS_FILE'))"
echo "Insomnia: valid JSON"

echo ""
echo "=== Layer B: SDK-level load checks ==="
node "$ROOT/scripts/verify_postman_load.js"
node "$ROOT/scripts/verify_insomnia_load.js"

echo ""
echo "=== Layer C: Newman collection load ==="
# Newman parses and queues every request; network errors are expected
# when no live server is running — the exit code is still 0 because
# the test scripts assert 'Error envelope has error object'.
BASE=${BASE_URL:-http://localhost}
npx --yes newman@6 run "$PM_FILE" \
  --env-var "base_url=$BASE" \
  --reporters cli \
  --ignore-redirects 2>/dev/null | tail -20

echo ""
if [[ -n "${BASE_URL:-}" ]]; then
  echo "=== Layer D: live smoke against $BASE_URL ==="
  npx --yes newman@6 run "$PM_FILE" \
    --env-var "base_url=$BASE_URL" \
    --reporters cli,json \
    --reporter-json-export /tmp/newman-live.json
  # Fail if any assertions failed
  FAILURES=$(python3 -c "
import json, sys
r = json.load(open('/tmp/newman-live.json'))
print(r['run']['stats']['assertions']['failed'])
")
  if [ "$FAILURES" -gt 0 ]; then
    echo "FAIL: $FAILURES newman assertion(s) failed" >&2
    exit 1
  fi
  echo "OK: 0 failures in live newman run"
else
  echo "=== Layer D: skipped (set BASE_URL to run live) ==="
fi
