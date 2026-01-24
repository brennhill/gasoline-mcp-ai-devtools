// Scene: Activity — POST payload sends user_id as string instead of number
// API returns 422 with validation error
// Gasoline captures both request and response bodies

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Activity Malformed Payload\n");

  const { context, page } = await launch();

  // Navigate to activity page — auto-triggers the bug on mount
  await page.goto("http://localhost:3000/activity");
  await pause(2000, "Activity page loads — auto-tracks event with wrong type");

  // The page auto-fires a POST with user_id as string, should see error
  await pause(2000, "Error message should be visible — 422 validation error");

  // Click the track button to trigger it again explicitly
  await page.click('[data-testid="track-event"]');
  await pause(2000, "Manual track event — same 422 error");

  console.log("\n  ✓ 422 validation error triggered — Gasoline should have captured request + response bodies\n");

  await context.close();
}
