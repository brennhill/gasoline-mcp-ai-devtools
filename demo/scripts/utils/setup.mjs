// Browser launch utilities for demo automation
import { chromium } from "playwright";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const extensionPath = path.resolve(__dirname, "../../../extension");

export async function launch() {
  const context = await chromium.launchPersistentContext("", {
    headless: false,
    slowMo: 80,
    viewport: { width: 1280, height: 720 },
    args: [
      `--disable-extensions-except=${extensionPath}`,
      `--load-extension=${extensionPath}`,
      "--window-size=1280,720",
      "--window-position=0,0",
    ],
  });

  // Get extension ID from service worker
  const extensionId = await getExtensionId(context);
  console.log(`  Extension ID: ${extensionId}`);

  const page = await context.newPage();
  return { context, page, extensionId };
}

async function getExtensionId(context) {
  // Wait for the service worker to register
  let sw;
  try {
    sw = context.serviceWorkers()[0] || await context.waitForEvent("serviceworker", { timeout: 5000 });
  } catch {
    console.warn("  Warning: Could not detect extension service worker");
    return "unknown";
  }
  return sw.url().split("/")[2];
}

export async function openPopup(context) {
  // Open the real extension popover via chrome.action.openPopup()
  const sw = context.serviceWorkers()[0];
  if (!sw) throw new Error("No service worker found — cannot open popup");

  // Ensure the active page is fully loaded and focused
  const activePage = context.pages().find((p) => p.url() !== "about:blank") || context.pages()[0];
  await activePage.bringToFront();
  await activePage.waitForLoadState("load");
  await new Promise((r) => setTimeout(r, 200));

  // Trigger the real popover
  await sw.evaluate(() => chrome.action.openPopup());

  // Give the popup time to render
  await new Promise((r) => setTimeout(r, 500));

  // Try to find the popup page by its URL
  const extensionId = sw.url().split("/")[2];
  const popupPage = context.pages().find((p) => p.url().includes(`${extensionId}/popup.html`));
  return popupPage || null;
}

export async function pause(ms, description) {
  if (description) {
    console.log(`  ⏸  ${description} (${ms}ms)`);
  }
  await new Promise((r) => setTimeout(r, ms));
}

export async function setExtensionSetting(context, settings) {
  // Set extension storage values via the service worker
  const sw = context.serviceWorkers()[0];
  if (!sw) throw new Error("No service worker found");
  await sw.evaluate((s) => chrome.storage.local.set(s), settings);
  await new Promise((r) => setTimeout(r, 100));
}

export async function clearGasoline(port = 7890) {
  try {
    await fetch(`http://localhost:${port}/logs`, { method: "DELETE" });
  } catch {
    // Server might not be running — non-fatal
  }
}

export async function typeNaturally(page, selector, text) {
  await page.fill(selector, "");
  for (const char of text) {
    await page.type(selector, char, { delay: 100 + Math.random() * 50 });
  }
}
