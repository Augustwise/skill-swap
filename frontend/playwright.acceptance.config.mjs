import { defineConfig } from "@playwright/test";
import { browserProjects } from "./tests/helpers/browser-projects.mjs";

export default defineConfig({
  testDir: "./tests/acceptance",
  testMatch: "*.spec.mjs",
  globalSetup: "./tests/acceptance/local-stack.mjs",
  workers: 1,
  retries: 0,
  timeout: 90_000,
  projects: browserProjects(),
  outputDir: "test-results/acceptance",
  reporter: [
    ["list"],
    ["html", { outputFolder: "playwright-report/acceptance", open: "never" }],
    ["json", { outputFile: "test-results/acceptance.json" }],
  ],
  use: {
    baseURL: "http://localhost:3000",
    timezoneId: "Europe/Kyiv",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
