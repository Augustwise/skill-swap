import { test as base, expect } from "@playwright/test";

export { expect };
export const test = base.extend({
  browserVersion: [
    async ({ browser, browserName }, use, testInfo) => {
      await testInfo.attach("browser-version", {
        body: JSON.stringify({
          project: testInfo.project.name,
          engine: browserName,
          version: browser.version(),
          platform: process.platform,
          date: new Date().toISOString(),
        }),
        contentType: "application/json",
      });
      await use();
    },
    { auto: true },
  ],
});
