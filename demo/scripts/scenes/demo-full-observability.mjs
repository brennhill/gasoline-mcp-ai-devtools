// Demo Script 4: "Full Observability"
// Enables all capture features, triggers multiple bugs, shows comprehensive capture

import { launch, pause, typeNaturally, openPopup, setExtensionSetting } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Demo: Full Observability\n");

  const { context, page } = await launch();

  // Step 1: Navigate to app
  await page.bringToFront();
  await page.goto("http://localhost:3000/");
  await pause(1500, "App loaded");

  // Step 2: Enable all capture features via storage
  await setExtensionSetting(context, {
    webSocketCaptureEnabled: true,
    networkWaterfallEnabled: true,
    networkBodyCaptureEnabled: true,
    actionReplayEnabled: true,
    logLevel: "all",
  });
  await pause(500, "All features enabled — maximum observability");

  // Step 3: Show popup with everything enabled
  await openPopup(context);
  await pause(2000, "Popup shows: All toggles ON, capture level = All Logs");
  await page.bringToFront();

  // Step 4: Trigger Users 500
  await page.goto("http://localhost:3000/users");
  await pause(1500, "→ Users page");
  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "500 error triggered — body + actions captured");

  // Step 5: Trigger Settings rejection
  await page.goto("http://localhost:3000/settings");
  await pause(1500, "→ Settings page");
  await page.click('[data-testid="save-button"]');
  await pause(2500, "Unhandled promise rejection — 503 response captured");

  // Step 6: Trigger Activity 422
  await page.goto("http://localhost:3000/activity");
  await pause(2000, "→ Activity page — auto-fires malformed POST");
  await pause(1500, "422 validation error — request + response bodies captured");

  // Step 7: Wait for all data to flush
  await pause(4000, "Extension flushing all captured data...");

  // Step 8: Show final popup state
  await openPopup(context);
  await pause(3000, "Popup: Multiple errors captured across 3 different bug types");

  console.log("\n  ✓ Full observability — every layer captured, AI sees everything\n");

  await context.close();
}
