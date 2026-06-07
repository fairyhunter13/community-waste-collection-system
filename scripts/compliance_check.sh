#!/usr/bin/env bash
# License and terminology audit — asserts that no identifying strings from
# the private requirements document leak into committed files. Searches the
# tree (excluding .git and binary/gitignored paths) for the forbidden
# patterns and exits non-zero if any are found.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FAILED=0

SCRIPT_NAME="$(basename "$0")"

check_pattern() {
  local pattern="$1"
  local description="$2"
  # Exclude this script itself (it contains the patterns as grep arguments),
  # the gitignored PDF, and REQUIREMENTS_RAW.
  if grep -rIl --exclude-dir=.git --exclude-dir=vendor \
      --exclude="*.pdf" --exclude="REQUIREMENTS_RAW*" \
      --exclude="$SCRIPT_NAME" \
      "$pattern" "$ROOT" 2>/dev/null | grep -v "^Binary "; then
    echo "ERROR: Found forbidden pattern ($description)" >&2
    FAILED=1
  fi
}

# The patterns below are the forbidden identifiers; they appear here only as
# grep arguments and are excluded from the search via --exclude="$SCRIPT_NAME".
check_pattern "INOS""OFT" "company name upper"
check_pattern "Inos""oft" "company name mixed"
check_pattern "inos""oft" "company name lower"

# Contact information
check_pattern "hrd@inos""oftweb" "HR email"
check_pattern "inos""oftweb\.com" "company domain"
check_pattern "Pand""ugo" "physical address"
check_pattern "+6231-8700688" "phone number"
check_pattern "6231.8700688" "phone number variant"

if [ "$FAILED" -eq 1 ]; then
  echo ""
  echo "Terminology audit FAILED: remove all forbidden strings before committing."
  exit 1
fi

echo "Terminology audit PASSED: no forbidden strings found."
