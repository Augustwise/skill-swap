import { expect } from "@playwright/test";

export async function measurePageLoads(page, routes) {
  const samples = [];
  for (let index = 0; index < 30; index++) {
    const { path, ready } = routes[index % routes.length];
    const started = performance.now();
    await page.goto(path, { waitUntil: "domcontentloaded" });
    await ready(page);
    await page.evaluate(
      () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
    );
    samples.push({ path, ms: performance.now() - started });
  }
  return samples;
}

export async function assertPageBudget(samples, testInfo, details = {}) {
  const sorted = samples.map(({ ms }) => ms).sort((a, b) => a - b);
  const p95 = sorted[Math.ceil(sorted.length * 0.95) - 1];
  const withinBudget = samples.filter(({ ms }) => ms <= 3000).length;
  await testInfo.attach("page-load-measurements", {
    body: JSON.stringify({ ...details, samples, p95, withinBudget }, null, 2),
    contentType: "application/json",
  });
  expect(samples).toHaveLength(30);
  expect(withinBudget / samples.length).toBeGreaterThanOrEqual(0.95);
  expect(p95).toBeLessThanOrEqual(3000);
}
