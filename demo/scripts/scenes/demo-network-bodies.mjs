// Demo Script 3: "Network Bodies On/Off"
// Shows the difference between capturing with and without response bodies

import { launch, pause, typeNaturally, openPopup, setExtensionSetting } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Demo: Network Bodies On/Off\n");

  const { context, page } = await launch();

  // Step 1: Navigate and show popup — Network Bodies is ON (default)
  await page.bringToFront();
  await page.goto("http://localhost:3000/users");
  await pause(1500, "Users page loaded");

  await openPopup(context);
  await pause(2000, "Popup shows: Network Bodies toggle is ON (default)");
  await page.bringToFront();

  // Step 2: Trigger error WITH bodies enabled
  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Search 'admin' — 500 error with FULL response body captured");

  // Wait for flush
  await pause(3000, "Extension sends data with response body...");

  // Step 3: Disable Network Bodies via storage
  await setExtensionSetting(context, { networkBodyCaptureEnabled: false });
  await pause(500, "Network Bodies toggled OFF");

  // Show popup with updated state
  await openPopup(context);
  await pause(1500, "Popup shows: Network Bodies is now OFF");
  await page.bringToFront();

  // Step 4: Trigger the same error WITHOUT bodies
  await page.goto("http://localhost:3000/users");
  await pause(1500, "Users page loaded again");

  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Search 'admin' — 500 error captured but WITHOUT response body");

  // Wait for flush
  await pause(3000, "Extension sends data without body (lighter payload)...");

  // Step 5: Show popup with updated counts
  await openPopup(context);
  await pause(3000, "Popup: 2 errors total — first has body, second doesn't");

  console.log("\n  ✓ Control what your AI sees — toggle bodies for privacy or context savings\n");

  await context.close();
}
