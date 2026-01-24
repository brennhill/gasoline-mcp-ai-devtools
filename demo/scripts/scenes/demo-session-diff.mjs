// Demo: Session Diff — Shows diff_sessions comparing before/after states
// Creates a checkpoint, makes changes, then diffs

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Session Diff (Before/After Comparison)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to a clean state
  await page.goto("http://localhost:3000/");
  await pause(2000, "Dashboard loaded — clean initial state");

  // Step 2: Navigate around (baseline activity)
  await page.goto("http://localhost:3000/users");
  await pause(1500, "Users page — no errors yet");

  await page.goto("http://localhost:3000/integrations");
  await pause(1500, "Integrations page — API calls succeed");

  // Wait for baseline to flush
  await pause(3000, "Baseline activity captured...");

  console.log("  --- CHECKPOINT: Take a snapshot of current state ---");
  console.log("  Call: configure({action:'store', store_action:'save', key:'before-bugs'})");
  await pause(2000);

  // Step 3: Now trigger bugs
  await page.goto("http://localhost:3000/users");
  await page.fill('[data-testid="search"]', "admin");
  await pause(2000, "Triggered 500 error on /users");

  await page.goto("http://localhost:3000/settings");
  await page.click("text=Save Changes");
  await pause(2000, "Triggered unhandled rejection on /settings");

  await page.goto("http://localhost:3000/activity");
  await pause(2000, "Triggered 422 validation error on /activity");

  // Wait for error flush
  await pause(3000, "Error activity captured...");

  console.log("  --- AFTER: State now has 3 new errors ---");
  console.log("  Call: configure({action:'store', store_action:'save', key:'after-bugs'})");
  await pause(1000);

  console.log("\n  \u2713 Two snapshots captured — before and after bugs\n");
  console.log("  Try: analyze({target:'changes', checkpoint:'before-bugs'})");
  console.log("  Gasoline will show:");
  console.log("    - 3 new errors appeared");
  console.log("    - 2 new network failures (500, 422)");
  console.log("    - 1 unhandled promise rejection");
  console.log("    - Exact diff of what changed between snapshots\n");

  await context.close();
}
