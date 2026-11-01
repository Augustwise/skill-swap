import { defineConfig } from "@playwright/test";
import { browserProjects } from "./tests/helpers/browser-projects.mjs";

const reportName = process.env.E2E_REPORT_NAME || "acceptance";
if (!/^[a-z0-9-]+$/.test(reportName)) throw new Error("Invalid acceptance report name");

export default defineConfig({
  testDir: "./tests/acceptance",
  testMatch: "*.spec.mjs",
  globalSetup: "./tests/acceptance/local-stack.mjs",
  workers: 1,
  retries: 0,
  timeout: 90_000,
  projects: browserProjects(),
  outputDir: `test-results/${reportName}`,
  reporter: [
    ["list"],
    ["html", { outputFolder: `playwright-report/${reportName}`, open: "never" }],
    ["json", { outputFile: `test-results/${reportName}.json` }],
  ],
  use: {
    baseURL: "http://localhost:3000",
    timezoneId: "Europe/Kyiv",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
