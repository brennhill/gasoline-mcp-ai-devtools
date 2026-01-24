// Demo: Generate Reproduction — Shows generate/reproduction creating a repro script
// Triggers a bug with specific user actions, then shows the generated script

import { launch, pause, typeNaturally } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Generate Reproduction Script\n");

  const { context, page } = await launch();

  // Step 1: Simulate a realistic user flow that leads to a bug
  await page.goto("http://localhost:3000/");
  await pause(1500, "User starts at dashboard");

  await page.goto("http://localhost:3000/users");
  await pause(1000, "Navigates to users page");

  // Step 2: User interaction that triggers the bug
  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Searches for 'admin' — triggers 500 error!");

  // Step 3: User tries to recover
  await page.fill('[data-testid="search"]', "");
  await pause(500);
  await typeNaturally(page, '[data-testid="search"]', "alice");
  await pause(1500, "Tries 'alice' — works fine");

  // Step 4: Reproduce the error again
  await page.fill('[data-testid="search"]', "");
  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Tries 'admin' again — same 500 error!");

  // Wait for flush
  await pause(3000, "All actions and errors captured...");

  console.log("\n  \u2713 Bug reproduction captured with full action history\n");
  console.log("  Try: generate({format:'reproduction', error_message:'500'})");
  console.log("  Gasoline generates a script like:");
  console.log("  ");
  console.log("    // Reproduction steps:");
  console.log("    // 1. Navigate to http://localhost:3000/users");
  console.log("    // 2. Type 'admin' into search field");
  console.log("    // 3. Observe: GET /api/users?q=admin returns 500");
  console.log("    await page.goto('http://localhost:3000/users');");
  console.log("    await page.fill('[data-testid=\"search\"]', 'admin');");
  console.log("    // Error: Internal Server Error");
  console.log("  ");
  console.log("  Try: generate({format:'test', test_name:'users-search-500'})");
  console.log("  Generates a full Playwright test case!\n");

  await context.close();
}
