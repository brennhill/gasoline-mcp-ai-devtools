// Scene: Notifications — WebSocket disconnects after 10 seconds
// Gasoline captures the WS lifecycle events

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Notifications WebSocket Disconnect\n");

  const { context, page } = await launch();

  // Navigate to notifications page
  await page.goto("http://localhost:3000/notifications");
  await pause(2000, "Notifications page loads, WebSocket connecting...");

  // Watch notifications arrive
  await pause(3000, "Notifications arriving in real-time...");
  await pause(3000, "More notifications flowing in...");

  // Wait for the disconnect (happens at 10s)
  await pause(5000, "Waiting for disconnect... status changes to 'Disconnected'");

  // Show the disconnect happened silently
  await pause(2000, "No error shown! Notifications just stopped. Silent failure.");

  console.log("\n  ✓ WebSocket disconnect triggered — Gasoline should have captured ws:close event\n");

  await context.close();
}
