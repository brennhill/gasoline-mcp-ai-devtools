// Demo: API Schema — Shows analyze/api detecting endpoint patterns
// Navigates through integrations page triggering multiple API calls

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: API Schema Discovery\n");

  const { context, page } = await launch();

  // Step 1: Navigate to integrations page (triggers GET /api/integrations + /stats)
  await page.goto("http://localhost:3000/integrations");
  await pause(2000, "Integrations page loaded — 2 API calls made (list + stats)");

  // Step 2: Test a connection (triggers GET /api/integrations/slack/test)
  await page.click("text=Test >> nth=0");
  await pause(1500, "Connection test — GET with query params");

  // Step 3: Sync an integration (triggers POST /api/integrations/slack/sync)
  await page.click("text=Sync Now >> nth=0");
  await pause(2000, "Sync triggered — POST with JSON body");

  // Step 4: Test another connection (different endpoint pattern)
  await page.click("text=Test >> nth=2");
  await pause(1500, "Jira test — returns error response (401)");

  // Step 5: Try to sync Jira (will fail)
  await page.click("text=Sync Now >> nth=2");
  await pause(2000, "Jira sync — fails with auth_expired error");

  // Step 6: Visit another page for more API patterns
  await page.goto("http://localhost:3000/users");
  await pause(1000);
  await page.fill('[data-testid="search"]', "test");
  await pause(2000, "User search — GET /api/users?q=test");

  // Step 7: Wait for flush
  await pause(3000, "All API calls captured by Gasoline...");

  console.log("\n  \u2713 Multiple API patterns observed\n");
  console.log("  Try: analyze({target:'api'}) — discovers endpoint schema");
  console.log("  Gasoline will show:");
  console.log("    - GET /api/integrations (list)");
  console.log("    - GET /api/integrations/stats");
  console.log("    - GET /api/integrations/:id/test?timeout&verbose");
  console.log("    - POST /api/integrations/:id/sync {force, since}");
  console.log("    - GET /api/users?q=...\n");

  await context.close();
}
