// Demo Script 2: "Toggle WebSocket Capture"
// Opens popup, enables WebSocket capture, then shows WS disconnect being caught

import { launch, pause, openPopup, setExtensionSetting } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Demo: Toggle WebSocket Capture\n");

  const { context, page } = await launch();

  // Step 1: Navigate to a page so the popup can open
  await page.bringToFront();
  await page.goto("http://localhost:3000/");
  await pause(1500, "App loaded — defaults active");

  // Step 2: Show popup with defaults (WebSocket OFF)
  await openPopup(context);
  await pause(2000, "Popup shows: Connected, WebSocket Capture OFF (default)");
  await page.bringToFront();

  // Step 3: Enable WebSocket Capture via storage
  await setExtensionSetting(context, { webSocketCaptureEnabled: true });
  await pause(500, "WebSocket Capture toggled ON");

  // Step 4: Show popup with updated state
  await openPopup(context);
  await pause(1500, "Popup shows: WebSocket Capture is now ON");
  await page.bringToFront();

  // Step 5: Navigate to notifications page
  await page.goto("http://localhost:3000/notifications");
  await pause(3000, "Notifications page — WebSocket connected, messages arriving");

  // Step 6: Wait for the abnormal disconnect (happens at 10s)
  await pause(8000, "Waiting for WebSocket abnormal disconnect (10s timeout)...");

  // Step 7: The disconnect happened — page shows error
  await pause(2000, "WebSocket disconnected with code 1006 — error visible on page");

  // Step 8: Show popup with updated error count
  await openPopup(context);
  await pause(3000, "Popup shows: Error count increased — WS disconnect captured");

  console.log("\n  ✓ WebSocket disconnect captured — one toggle, full visibility\n");

  await context.close();
}
