import { defineConfig } from "@playwright/test";
import base from "./playwright.config.mjs";
import { browserProjects } from "./tests/helpers/browser-projects.mjs";

const sharedUse = { ...base.use };
delete sharedUse.channel;

export default defineConfig({
  ...base,
  projects: browserProjects(),
  outputDir: "test-results/matrix",
  reporter: [
    ["list"],
    ["html", { outputFolder: "playwright-report/matrix", open: "never" }],
    ["json", { outputFile: "test-results/matrix.json" }],
  ],
  use: { ...sharedUse, trace: "retain-on-failure", screenshot: "only-on-failure" },
});
