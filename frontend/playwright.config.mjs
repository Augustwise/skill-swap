import { existsSync } from "node:fs";
import { chromium, defineConfig } from "@playwright/test";

function browserChannel() {
  if (process.env.PLAYWRIGHT_CHANNEL) return process.env.PLAYWRIGHT_CHANNEL;
  try {
    if (existsSync(chromium.executablePath())) return undefined;
  } catch {
    // Fall back to an installed browser when Playwright's bundled binary is absent.
  }
  const candidates = [
    ["chrome", "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"],
    ["chrome", "C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe"],
    ["chrome", "/usr/bin/google-chrome"],
    ["msedge", "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe"],
    ["msedge", "C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe"],
  ];
  return candidates.find(([, path]) => existsSync(path))?.[0];
}

export default defineConfig({
  testDir: "./tests/browser",
  workers: 1,
  use: {
    baseURL: "http://127.0.0.1:3100",
    timezoneId: "Europe/Kyiv",
    channel: browserChannel(),
  },
  webServer: {
    command: "node node_modules/next/dist/bin/next start --hostname 127.0.0.1 --port 3100",
    url: "http://127.0.0.1:3100",
    timeout: 30000,
  },
});
