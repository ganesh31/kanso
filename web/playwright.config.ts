import { defineConfig, devices } from "@playwright/test";
import { tmpdir } from "node:os";
import { join } from "node:path";

// The interviewer flow (spec §10) runs against the built server (`make build`) in demo mode, on a
// database built from the committed Parquet fixture: no network, no Yahoo, no ingested kanso.db.
const port = process.env.KANSO_E2E_PORT ?? "8099";
const db = join(tmpdir(), "kanso-e2e.db");

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: process.env.CI ? [["list"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    cwd: "..",
    command: [
      `rm -f '${db}' '${db}-wal' '${db}-shm'`,
      `go run ./cmd/export-fixture -import testdata/benchmark/bars.parquet -db '${db}'`,
      `./bin/server -demo -db '${db}' -addr 127.0.0.1:${port}`,
    ].join(" && "),
    url: `http://127.0.0.1:${port}/api/universes`,
    env: { CGO_ENABLED: "0" },
    reuseExistingServer: false,
    timeout: 120_000,
  },
});
