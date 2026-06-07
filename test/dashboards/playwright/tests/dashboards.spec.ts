import { test, expect, Page } from "@playwright/test";
import * as fs from "fs";
import * as path from "path";

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/** Glob all dashboard JSON files from the deployments directory. */
function loadDashboards(): Array<{ uid: string; title: string }> {
  // Walk up from the playwright directory to find the repo root.
  const repoRoot = path.resolve(__dirname, "..", "..", "..", "..", "..");
  const dashDir = path.join(
    repoRoot,
    "deployments",
    "grafana",
    "dashboards"
  );
  const files = fs
    .readdirSync(dashDir)
    .filter((f) => f.endsWith(".json"))
    .map((f) => {
      const raw = JSON.parse(fs.readFileSync(path.join(dashDir, f), "utf-8"));
      return { uid: raw.uid as string, title: raw.title as string };
    });
  return files;
}

/** Wait until all loading spinners disappear and no error states remain. */
async function waitForPanelsReady(page: Page, timeout = 60_000) {
  // Grafana renders panels asynchronously; wait until no spinner remains.
  await page.waitForFunction(
    () =>
      document.querySelectorAll('[data-testid="panel-loading-bar"]').length ===
      0,
    { timeout }
  );
}

// Error phrases that indicate a panel failed to load.
const ERROR_PHRASES = [
  "Datasource error",
  "Query error",
  "No data",
  "datasource not found",
];

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

const dashboards = loadDashboards();

for (const { uid, title } of dashboards) {
  test(`dashboard "${title}" renders without errors`, async ({ page }) => {
    // Navigate in kiosk mode so the full panel grid is visible.
    await page.goto(`/d/${uid}?orgId=1&kiosk&refresh=10s`);
    await page.waitForSelector('[data-testid="dashboard-panels"]', {
      timeout: 30_000,
    });
    await waitForPanelsReady(page);

    // Screenshot archived as CI artefact by the workflow.
    await page.screenshot({
      path: `test-results/${uid}.png`,
      fullPage: true,
    });

    // Assert no panel body contains an error phrase.
    for (const phrase of ERROR_PHRASES) {
      const count = await page
        .locator('[data-testid^="panel-"]')
        .filter({ hasText: phrase })
        .count();
      expect(count, `panel with "${phrase}" found`).toBe(0);
    }

    // Assert at least one panel rendered a canvas or SVG element (proof of
    // non-trivial rendering — a dashboard of all-blank panels would fail here).
    const graphElements = await page
      .locator("canvas, svg")
      .count();
    expect(
      graphElements,
      "no canvas/SVG elements found — all panels may be blank"
    ).toBeGreaterThan(0);
  });
}
