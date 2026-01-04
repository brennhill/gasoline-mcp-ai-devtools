// Scene: Settings — Unhandled promise rejection on save
// Gasoline captures the unhandled rejection in console errors

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Settings Unhandled Rejection\n");

  const { context, page } = await launch();

  // Navigate to settings page
  await page.goto("http://localhost:3000/settings");
  await pause(2000, "Settings page loads with form");

  // Change a field
  await page.fill('input[type="text"]', "Updated User Name");
  await pause(1000, "User updates their display name");

  // Click save — triggers unhandled promise rejection
  await page.click('[data-testid="save-button"]');
  await pause(2000, "Save button clicked — nothing visible happens!");

  // Wait a moment to let the promise reject
  await pause(1500, "Button appears to do nothing... but an error occurred");

  console.log("\n  ✓ Unhandled promise rejection triggered — Gasoline should have captured console error\n");

  await context.close();
}
