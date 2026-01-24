// Demo Script 1: "Zero Config" — Automatic capture with no setup
// Shows popup before (0 errors) and after (1 error) to prove it just works

import { launch, pause, typeNaturally, openPopup } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Demo: Zero Config (Automatic Capture)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to app first so page is loaded
  await page.bringToFront();
  await page.goto("http://localhost:3000/users");
  await pause(1500, "Users page loaded");

  // Step 2: Show popup BEFORE — connected, 0 errors
  await openPopup(context);
  await pause(2500, "Popup shows: Connected, 0 errors — no setup needed");
  await page.bringToFront(); // dismiss popover

  // Step 3: Trigger the error
  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Search 'admin' — 500 error appears on page");

  // Step 4: Wait for extension to flush
  await pause(3000, "Extension batches and sends data to server...");

  // Step 5: Show popup AFTER — error count incremented
  await openPopup(context);
  await pause(3000, "Popup shows: Error count is now 1 — captured automatically!");

  console.log("\n  ✓ Error captured with zero configuration — just install and go\n");

  await context.close();
}
