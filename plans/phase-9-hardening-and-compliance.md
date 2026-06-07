# Phase 9 — Hardening & Compliance

## Purpose

Close the remaining supply-chain, API-contract, load-testing, and compliance
gaps that a security-focused reviewer would flag. None of these tasks touch
the core business logic or the 16 spec endpoints — they are all additive
hardening and sign-off infrastructure.

The phase is organised into four tiers:
- **Tier 8** — supply-chain & security hardening (CVE scanning, SBOM, GitHub hygiene)
- **Tier 9** — API contract & DX polish (Postman assertions, OpenAPI examples, hot-reload)
- **Tier 10** — realistic load testing (k6 suite, SLO thresholds, CI workflow)
- **Tier 12** — compliance & sign-off (traceability tables, CHANGELOG, final CI gate)

The PDF brief and `REQUIREMENTS_RAW.md` remain gitignored. No company name,
email, address, or phone number from the backend engineering test brief is
permitted in any committed file.

---

## Tier 8 — Supply-Chain & Security Hardening (T57–T63)

### T57 — govulncheck CI job

Add a `vuln` job to `.github/workflows/ci.yml` that runs after `lint` and
before `test-unit`. Uses the official `golang/govulncheck-action`:

```yaml
# .github/workflows/ci.yml
  vuln:
    runs-on: ubuntu-latest
    needs: lint
    steps:
      - uses: actions/checkout@v4
      - uses: golang/govulncheck-action@v1
        with:
          go-version-input: "1.26"
          check-latest: true
          go-package: ./...
```

The action exits non-zero on any HIGH or CRITICAL finding.
`golang.org/x/vuln` is the underlying tool; no extra `go get` is required
because the action bundles it. Add `vuln` to the `needs:` array of the
existing `e2e` job so E2E does not run against a vulnerable dependency tree.

### T58 — Trivy image scan CI job

Add an `image-scan` job that builds the production Docker image and scans it
with Aqua Security's Trivy:

```yaml
  image-scan:
    runs-on: ubuntu-latest
    needs: vuln
    steps:
      - uses: actions/checkout@v4
      - name: Build image
        run: docker build -f build/Dockerfile -t waste-api:ci .
      - uses: aquasecurity/trivy-action@master
        with:
          image-ref: waste-api:ci
          format: table
          exit-code: "1"
          ignore-unfixed: true
          severity: HIGH,CRITICAL
```

Fails the job on any HIGH or CRITICAL CVE that has a fix available
(`ignore-unfixed: true` silences CVEs that have no upstream patch, avoiding
false-positive noise). The image build here reuses exactly the same
`build/Dockerfile` that runs in production — no separate scan image.

### T59 — Syft SBOM generation

Append a third step to the `image-scan` job to generate a CycloneDX JSON SBOM
and upload it as a CI artifact:

```yaml
      - name: Generate SBOM (Syft)
        uses: anchore/sbom-action@v0
        with:
          image: waste-api:ci
          artifact-name: sbom.cyclonedx.json
          format: cyclonedx-json
```

The artifact is retained for 30 days (default). No secrets are required.
`anchore/sbom-action` pulls Syft automatically.

### T60 — `SECURITY.md`

New file `SECURITY.md` at the repository root. Content: supported versions
table (current `main` branch only), and a pointer to GitHub Security Advisories
(`https://github.com/<owner>/<repo>/security/advisories`) for private
vulnerability disclosure. No email address, phone number, or company
identifying information.

```markdown
## Supported Versions

| Branch | Supported |
|--------|-----------|
| main   | ✅        |

## Reporting a Vulnerability

Please use [GitHub Security Advisories](<url>) to report vulnerabilities
privately. Do not open a public issue for security findings.
```

### T61 — `.github/CODEOWNERS`

New file `.github/CODEOWNERS`:

```
# Default reviewer for all files
*  @fairyhunter13
```

Add path-specific rules for the critical paths:

```
internal/service/   @fairyhunter13
internal/handler/   @fairyhunter13
migrations/         @fairyhunter13
deployments/        @fairyhunter13
.github/workflows/  @fairyhunter13
```

GitHub enforces a review from the listed owner before any PR can merge.

### T62 — Issue and PR templates

New files:

**`.github/ISSUE_TEMPLATE/bug.md`**

```markdown
---
name: Bug report
about: Something is not working as described in the backend engineering test brief
labels: bug
---

## Describe the bug
<!-- A clear and concise description of what is wrong -->

## Steps to reproduce
1.
2.

## Expected behaviour
<!-- What you expected to happen -->

## Actual behaviour
<!-- What actually happened, including error envelope body if applicable -->

## Environment
- Go version:
- Docker compose version:
- Branch / commit:
```

**`.github/ISSUE_TEMPLATE/feature.md`**

```markdown
---
name: Feature request
about: Suggest an improvement or addition
labels: enhancement
---

## Problem this feature solves

## Proposed solution

## Alternatives considered
```

**`.github/PULL_REQUEST_TEMPLATE.md`**

```markdown
## Summary

## Type of change
- [ ] Bug fix
- [ ] New feature
- [ ] Refactor / cleanup
- [ ] Documentation
- [ ] CI / infrastructure

## Checklist
- [ ] `golangci-lint run ./...` passes locally
- [ ] `go test -race ./...` passes
- [ ] Coverage gate still ≥ 80%
- [ ] No company name / PII added to any file
- [ ] Conventional commit message format used
- [ ] Phase plan updated if behaviour changed
```

### T63 — `.editorconfig`

New file `.editorconfig` at the repository root. Codifies the conventions
already enforced by `golangci-lint` (tabs for Go, final newline everywhere):

```ini
root = true

[*]
end_of_line   = lf
insert_final_newline = true
trim_trailing_whitespace = true
charset = utf-8

[*.go]
indent_style = tab
indent_size  = 4

[*.{yaml,yml,json,md}]
indent_style = space
indent_size  = 2

[Makefile]
indent_style = tab
```

---

## Tier 9 — API Contract & DX Polish (T64–T67)

### T64 — `pm.test` assertions in every Postman request

Every request in `api/community-waste.postman_collection.json` currently has
either no `Tests` script or a minimal status-code check. Add a shared `Tests`
block to each:

```javascript
// Minimum assertions on every request
pm.test("Status code is in expected range", function () {
    pm.expect(pm.response.code).to.be.oneOf([200, 201, 204, 400, 404, 409, 422]);
});
pm.test("Content-Type is application/json", function () {
    pm.expect(pm.response.headers.get("Content-Type")).to.include("application/json");
});
pm.test("success key is present", function () {
    const json = pm.response.json();
    pm.expect(json).to.have.property("success");
});
pm.test("Response time is under 1000ms", function () {
    pm.expect(pm.response.responseTime).to.be.below(1000);
});
```

Creator endpoints (`POST /api/households`, `POST /api/pickups`,
`POST /api/payments`) additionally assert:

```javascript
pm.test("Response data.id is UUID-shaped", function () {
    const id = pm.response.json().data.id;
    pm.expect(id).to.match(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i);
});
```

The `newman` CI job (T13, already in `.github/workflows/ci.yml`) picks up
these assertions automatically because it runs the same collection file.

### T65 — Insomnia collection parity

Ensure `api/community-waste.insomnia_collection.json` (created in T15 of
Phase 6) has the same 27+ requests as the Postman collection. If it was
generated via `npx insomnia-importers` it may be missing the three endpoints
added later (T9: `/readyz`, T48: `/api/version`, the `/metrics` smoke).
Update by hand or re-import from the latest Postman collection.

Verification: open the file, `jq '[.resources[] | select(.type=="request")] | length'`
— must be ≥ 27.

### T66 — `example:` on every OpenAPI component

Every request body, every 2xx response schema, and every 4xx error envelope in
`api/openapi.yaml` must carry at least one concrete `example:` value. Audit
the file for any `schema:` block without a sibling `example:` and add one.

Key places that are commonly missing examples:

- `POST /api/pickups` request body: add example with `type: "organic"`.
- `PUT /api/pickups/{id}/complete` 200 response: show the payment-created body.
- `PUT /api/payments/{id}/confirm` 422 response: show the `error.details[]` shape.
- All `400`/`404`/`409` responses: show the `{success, error: {code, message}}` shape.

Run `npx @redocly/cli lint api/openapi.yaml` after editing — the `redocly`
lint step in the CI `lint` job will catch any schema violations introduced
during the edit.

### T67 — Hot-reload with Air + `make dev`

New file `.air.toml` at the repository root:

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  args_bin = []
  bin = "./tmp/main"
  cmd = "go build -o ./tmp/main ./cmd/api"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "test", "deployments"]
  exclude_file = []
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "tpl", "tmpl", "html"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

Add to `Makefile`:

```makefile
## dev: start API with hot-reload (requires Air: go install github.com/air-verse/air@latest)
.PHONY: dev
dev:
	@air -c .air.toml
```

Document in `CONTRIBUTING.md` under a new "Hot-Reload Development" section:
how to install Air (`go install github.com/air-verse/air@latest`), run
`make dev`, and what to expect (automatic rebuild on `.go` file save).

---

## Tier 10 — Realistic Load Testing (T68–T71)

### T68 — k6 load test suite

New directory `test/load/` with four scripts. All share the same golden flow:
**create household → create pickup → schedule pickup → complete pickup →
confirm payment with proof**.

```
test/load/
├── smoke.js       — 1 VU, 1 min — fast sanity check
├── average.js     — 20 VU, 5 min — baseline performance
├── stress.js      — ramp to 100 VU over 10 min — find the knee
├── spike.js       — 0 → 200 → 0 VU in 2 min — sudden traffic burst
└── lib/
    ├── flow.js    — shared golden flow implementation
    └── thresholds.js — shared SLO threshold constants
```

Golden flow (`test/load/lib/flow.js`):

```javascript
import http from 'k6/http';
import { check } from 'k6';

const BASE = __ENV.BASE_URL || 'http://localhost:8080';

export function goldenFlow() {
  // 1. Create household
  let hRes = http.post(`${BASE}/api/households`,
    JSON.stringify({name: `load-${Date.now()}`, address: "123 Test St"}),
    {headers: {'Content-Type': 'application/json'}});
  check(hRes, {'household 201': r => r.status === 201});
  const hID = hRes.json('data.id');

  // 2. Create pickup
  let pRes = http.post(`${BASE}/api/pickups`,
    JSON.stringify({household_id: hID, type: 'organic'}),
    {headers: {'Content-Type': 'application/json'}});
  check(pRes, {'pickup 201': r => r.status === 201});
  const pickupID = pRes.json('data.id');

  // 3. Schedule
  let sRes = http.put(`${BASE}/api/pickups/${pickupID}/schedule`,
    JSON.stringify({pickup_date: '2027-01-01'}),
    {headers: {'Content-Type': 'application/json'}});
  check(sRes, {'schedule 200': r => r.status === 200});

  // 4. Complete (creates payment)
  let cRes = http.put(`${BASE}/api/pickups/${pickupID}/complete`, null);
  check(cRes, {'complete 200': r => r.status === 200});
  const payID = cRes.json('data.payment_id');

  // 5. Confirm with proof (multipart)
  const proof = http.file(open('../../fixtures/proof.jpg'), 'proof.jpg', 'image/jpeg');
  let fRes = http.put(`${BASE}/api/payments/${payID}/confirm`,
    {proof: proof});
  check(fRes, {'confirm 200': r => r.status === 200});
}
```

Smoke script (`test/load/smoke.js`):

```javascript
import { goldenFlow } from './lib/flow.js';
import { thresholds } from './lib/thresholds.js';

export const options = {
  vus: 1, duration: '1m',
  thresholds,
};
export default goldenFlow;
```

Average and stress scripts follow the same pattern with different `options`.
Spike script uses a k6 `stages` array:

```javascript
export const options = {
  stages: [
    {duration: '30s', target: 200},
    {duration: '1m',  target: 200},
    {duration: '30s', target: 0},
  ],
  thresholds,
};
```

### T69 — SLO thresholds (`test/load/lib/thresholds.js`)

Single source of truth for all four scripts:

```javascript
export const thresholds = {
  // Latency
  'http_req_duration{expected_response:true}': [
    'p(95)<300',
    'p(99)<800',
  ],
  // Error rate
  'http_req_failed': ['rate<0.01'],
  // Check pass rate
  'checks': ['rate>0.99'],
};
```

p95 < 300 ms, p99 < 800 ms, error rate < 1%. These mirror the Prometheus
alert thresholds defined in T14 of Phase 6 (`HighLatencyP99: > 1.0s`,
`HighErrorRate: > 5%` — the load test thresholds are deliberately stricter
as a pre-prod gate).

### T70 — `make load` and `.github/workflows/load.yml`

Makefile targets:

```makefile
## load-smoke: run k6 smoke test (1 VU, 1 min) against running stack
.PHONY: load-smoke
load-smoke: docker-up
	k6 run test/load/smoke.js

## load-average: run k6 average-load test (20 VU, 5 min)
.PHONY: load-average
load-average: docker-up
	k6 run test/load/average.js

## load: alias for smoke + average (CI-safe subset)
.PHONY: load
load: load-smoke load-average
```

GitHub Actions workflow `.github/workflows/load.yml`:

```yaml
name: Load Tests
on:
  workflow_dispatch:
    inputs:
      scenario:
        description: 'Scenario to run (smoke|average|stress|spike)'
        required: false
        default: smoke

jobs:
  load:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: {go-version: "1.26"}
      - name: Boot stack
        run: make docker-up
      - name: Wait for healthy
        run: |
          for i in $(seq 1 30); do
            curl -fsS http://localhost:8080/health && break
            sleep 2
          done
      - uses: grafana/setup-k6-action@v1
      - name: Run load test
        run: k6 run test/load/${{ github.event.inputs.scenario }}.js
      - name: Archive results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: k6-results-${{ github.event.inputs.scenario }}
          path: k6-results/
      - name: Tear down stack
        if: always()
        run: make docker-down
```

The workflow is `workflow_dispatch` only — it is never triggered automatically
so it cannot slow down the main CI pipeline.

### T71 — Grafana load-correlation panel

Add a **"Load Test Correlation"** row to the existing API dashboard
(`deployments/grafana/dashboards/waste-collection.json`):

| Panel | Type | Query |
|---|---|---|
| Requests per second | Time series | `rate(http_requests_total[1m])` |
| p95 / p99 latency | Time series | `histogram_quantile(0.95/0.99, rate(http_request_duration_seconds_bucket[1m]))` |
| Worker cycle rate | Stat | `rate(worker_cycles_total[5m])` |
| DB pool in-use | Time series | `db_pool_connections{state="inuse"}` |
| Rate-limiter active clients | Stat | `rate_limit_active_clients` |

The row is collapsed by default and is only meaningful when load tests are
running. No existing panels are removed.

---

## Tier 12 — Compliance & Sign-Off (T79–T82)

### T79 — `scripts/compliance_check.sh` and `make compliance-check`

New file `scripts/compliance_check.sh`:

```bash
#!/usr/bin/env bash
# compliance_check.sh — assert no company-identifying strings are in tracked files.
# Populate FORBIDDEN with substrings derived from the backend engineering test
# brief (company name fragments, HR email domain, address keywords, phone digits).
# Add terms here when the brief is updated.
set -euo pipefail

# NOTE: populate this array from the brief before running in a real engagement.
# The actual substrings are intentionally omitted from this plan file.
FORBIDDEN=(
  # "<company-name-fragment>"
  # "<hr-email-domain>"
  # "<address-keyword>"
  # "<phone-digit-sequence>"
)

FAILED=0
for term in "${FORBIDDEN[@]}"; do
  if git grep -rIi --include="*.go" --include="*.yaml" --include="*.yml" \
       --include="*.json" --include="*.md" --include="*.toml" \
       --include="*.sh" --include="*.env*" -l "$term" 2>/dev/null; then
    echo "FAIL: found forbidden term '$term' in tracked files"
    FAILED=1
  fi
done

if [ "$FAILED" -eq 0 ]; then
  echo "OK: no forbidden terms found"
fi
exit "$FAILED"
```

Make it executable (`chmod +x scripts/compliance_check.sh`).

Makefile target:

```makefile
## compliance-check: assert no company PII in tracked files
.PHONY: compliance-check
compliance-check:
	@bash scripts/compliance_check.sh
```

Wire into the CI `lint` job so it runs on every push:

```yaml
# .github/workflows/ci.yml — inside the lint job's steps
      - name: Compliance check
        run: make compliance-check
```

### T80 — Phase-7 final verification — traceability tables

Populate `plans/phase-7-final-verification.md` §1 with four fully filled
traceability tables. The three existing tables (endpoints, BRs, tech reqs)
should reference the actual `file:line` for each implementation and the
exact test function name (not just the file) for the test column. Add a
**fourth table** for the five deliverables:

| # | Deliverable | File or artefact | Status |
|---|---|---|---|
| D1 | Source code with all 16 endpoints | `internal/handler/`, `internal/service/` | ✅ |
| D2 | Single-command Docker Compose | `deployments/docker-compose.yml` | ✅ |
| D3 | Postman / Insomnia collections | `api/community-waste.postman_collection.json`, `api/community-waste.insomnia_collection.json` | ✅ |
| D4 | OpenAPI spec | `api/openapi.yaml` | ✅ |
| D5 | README with setup + architecture | `README.md` | ✅ |

The table is the final sign-off artefact — a reviewer should be able to open
any row, navigate to the referenced file and line, and immediately find the
code or test that satisfies the spec item.

### T81 — `CHANGELOG.md` Phase 9 section

Append a `## [Phase 9] — Hardening & Compliance` section to `CHANGELOG.md`
(which was created in T46 of Phase 6). Follow the same keep-a-changelog
format already in use:

```markdown
## [Phase 9] — Hardening & Compliance

### Added
- `govulncheck` CI job — fails on HIGH/CRITICAL CVEs (T57)
- Trivy image scan CI job with Syft CycloneDX SBOM artifact (T58/T59)
- `SECURITY.md`, `.github/CODEOWNERS`, issue/PR templates (T60–T62)
- `.editorconfig` for tabs/newline consistency (T63)
- `pm.test` assertions on all 27+ Postman requests (T64)
- Insomnia collection parity update (T65)
- `example:` values on every OpenAPI request/response (T66)
- `.air.toml` + `make dev` hot-reload target (T67)
- k6 load test suite: smoke, average, stress, spike + SLO thresholds (T68/T69)
- `make load`, `make load-smoke`, `make load-average` + `workflows/load.yml` (T70)
- Grafana load-correlation panel in API dashboard (T71)
- `scripts/compliance_check.sh` + `make compliance-check` + CI step (T79)
- Phase-7 traceability tables updated with file:line + test function names (T80)
```

### T82 — Push and watch CI

```bash
git add .
git commit -m "feat(phase-9): supply-chain, load tests, compliance check, traceability"
git push origin main
gh run watch --exit-status
```

All CI jobs must be green before declaring Phase 9 complete:

| Job | Expected |
|---|---|
| lint (incl. `compliance-check`) | ✅ |
| vuln | ✅ |
| image-scan (incl. SBOM) | ✅ |
| test-unit | ✅ |
| test-integration | ✅ |
| coverage-gate | ✅ |
| e2e | ✅ |
| perf | ✅ |
| contract (Newman) | ✅ |

---

## Verification

### Supply-chain (T57–T59)

```bash
# govulncheck locally (requires: go install golang.org/x/vuln/cmd/govulncheck@latest)
govulncheck ./...
# expected: "No vulnerabilities found." or only LOW findings

# Trivy locally (requires: brew install aquasecurity/trivy/trivy  or  apt-get install trivy)
docker build -f build/Dockerfile -t waste-api:local .
trivy image --severity HIGH,CRITICAL --ignore-unfixed waste-api:local
# expected: exit 0, no HIGH/CRITICAL rows

# SBOM generation locally (requires: brew install syft  or  curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh)
syft waste-api:local -o cyclonedx-json=sbom.cyclonedx.json
jq '.components | length' sbom.cyclonedx.json
# expected: positive integer (number of packages in the image)
```

### API contract (T64–T65)

```bash
# Newman with test assertions
make docker-up
newman run api/community-waste.postman_collection.json \
  --environment api/community-waste.postman_environment.json \
  --bail \
  --reporters cli
# expected: all requests pass, all pm.test assertions green, no failures

# Insomnia collection completeness check
jq '[.resources[] | select(.type=="request")] | length' \
  api/community-waste.insomnia_collection.json
# expected: ≥ 27

# OpenAPI lint (catches missing examples)
npx @redocly/cli lint api/openapi.yaml
# expected: 0 errors
```

### Hot-reload (T67)

```bash
# Install Air if not present
go install github.com/air-verse/air@latest

# Start hot-reload dev server (requires .env configured)
make dev
# expected: "running..." in terminal; edit any .go file → automatic rebuild within 1-2s
```

### Load tests (T68–T70)

```bash
# Requires k6: https://k6.io/docs/get-started/installation/
make docker-up
sleep 30

# Smoke test (1 VU, 1 min) — must pass SLO thresholds
k6 run test/load/smoke.js
# expected: all thresholds green; p95 < 300ms; error rate < 1%

# Average load (20 VU, 5 min)
k6 run test/load/average.js
# expected: same SLO thresholds; watch Grafana load-correlation panel live

# Verify Grafana load panel shows data during the run
open http://localhost:3000/d/waste-collection/api-dashboard
# expected: RPS, p95/p99, worker cycle rate panels show non-zero values
```

### Compliance (T79)

```bash
# Run locally before pushing
make compliance-check
# expected: "OK: no forbidden terms found"

# Double-check with raw grep (populate the pattern with the actual substrings
# from the backend engineering test brief before running)
git grep -rIi "<company-fragment-1>\|<company-fragment-2>\|<address-keyword>\|<email-domain>" -- \
  '*.go' '*.yaml' '*.yml' '*.json' '*.md' '*.toml' '*.sh'
# expected: no output (empty)
```

### Final CI gate (T82)

```bash
git push origin main
gh run watch --exit-status
# expected: all 9 jobs green; no job skipped
```
