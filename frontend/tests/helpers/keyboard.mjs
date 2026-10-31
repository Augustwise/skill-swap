import { expect } from "@playwright/test";

export async function tabTo(page, target) {
  await expect(target).toBeVisible();
  for (let count = 0; count < 100; count++) {
    if (await target.evaluate((element) => element === document.activeElement)) {
      await expect(target).toBeFocused();
      const visibleFocus = await target.evaluate((element) => {
        const candidates = [element];
        if (
          element.matches('input[type="radio"], input[type="checkbox"]') &&
          element.nextElementSibling
        )
          candidates.push(element.nextElementSibling);
        return candidates.some((candidate) => {
          const style = getComputedStyle(candidate);
          return (
            (style.outlineStyle !== "none" && parseFloat(style.outlineWidth) > 0) ||
            style.boxShadow !== "none"
          );
        });
      });
      expect(visibleFocus, "Keyboard focus must be visible").toBe(true);
      return;
    }
    await page.keyboard.press("Tab");
  }
  throw new Error(
    `Control cannot be reached with Tab: ${await target.evaluate((element) => element.outerHTML)}`,
  );
}

export async function type(page, target, value) {
  await tabTo(page, target);
  await page.keyboard.press("ControlOrMeta+A");
  await page.keyboard.type(value);
}

export async function activate(page, target) {
  await tabTo(page, target);
  await page.keyboard.press("Enter");
}

export async function assertLayout(page) {
  const metrics = await page.evaluate(() => ({
    lang: document.documentElement.lang,
    width: innerWidth,
    content: document.documentElement.scrollWidth,
  }));
  expect(metrics.lang).toBe("uk");
  expect(metrics.content).toBeLessThanOrEqual(metrics.width);
}
