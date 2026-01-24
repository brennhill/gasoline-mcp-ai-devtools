// Demo: Checkpoint — Shows analyze/changes tracking what happened since a point in time
// Saves a checkpoint, triggers activity, then analyzes changes

import { launch, pause, typeNaturally } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Checkpoint & Changes (Time-Travel Debugging)\n");

  const { context, page } = await launch();

  // Step 1: Clean state
  await page.goto("http://localhost:3000/");
  await pause(2000, "Dashboard loaded — this is our starting point");

  console.log("  --- CREATE CHECKPOINT ---");
  console.log("  The AI calls: analyze({target:'changes'}) with no checkpoint");
  console.log("  This creates a named checkpoint of current buffer positions");
  await pause(2000);

  // Step 2: User does things (captured as actions)
  await page.goto("http://localhost:3000/users");
  await pause(1000);
  await typeNaturally(page, '[data-testid="search"]', "bob");
  await pause(1500, "User searched for 'bob' — action captured");

  await page.goto("http://localhost:3000/integrations");
  await pause(1000);
  await page.click("text=Sync Now >> nth=0");
  await pause(2000, "User synced Slack integration — API call captured");

  await page.goto("http://localhost:3000/analytics");
  await pause(2000, "Analytics loaded — third-party requests captured");

  // Step 3: Trigger an error
  await page.goto("http://localhost:3000/users");
  await page.fill('[data-testid="search"]', "admin");
  await pause(2000, "500 error triggered!");

  // Wait for flush
  await pause(3000, "All activity flushed to server...");

  console.log("\n  \u2713 Activity recorded since checkpoint\n");
  console.log("  Try: analyze({target:'changes', checkpoint:'my-checkpoint'})");
  console.log("  Gasoline will show everything that happened SINCE the checkpoint:");
  console.log("    + 1 new error (500 on /api/users?q=admin)");
  console.log("    + 5 new network requests");
  console.log("    + 3 new user actions (search, click, navigation)");
  console.log("    + 2 new console entries");
  console.log("    + Third-party requests to analytics origins\n");

  await context.close();
}
