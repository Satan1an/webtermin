import { defineConfig, devices } from "@playwright/test";

// The panel runs in the stand container; run.sh starts Playwright with
// --network container:<stand>, so 127.0.0.1 is the stand itself.
export const BASE_URL = process.env.WT_BASE_URL ?? "https://127.0.0.1:8443";

export default defineConfig({
  testDir: "./tests",
  // One real server with shared state (first-run setup, users): run serially.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  timeout: 60_000,
  expect: { timeout: 15_000 },
  reporter: [["list"], ["html", { open: "never", outputFolder: "playwright-report" }]],
  use: {
    baseURL: BASE_URL,
    ignoreHTTPSErrors: true, // self-signed cert generated on first start
    // Mutating auth endpoints check Origin against the panel's own origin.
    extraHTTPHeaders: { Origin: BASE_URL },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    // First-run setup must happen before anything can log in.
    { name: "setup", testMatch: /.*\.setup\.ts/ },
    {
      name: "e2e",
      testMatch: /.*\.spec\.ts/,
      dependencies: ["setup"],
      use: { ...devices["Desktop Chrome"], ignoreHTTPSErrors: true },
    },
  ],
});
