// Scene: Users page — API returns 500 when searching "admin"
// Gasoline captures the network error with full response body

import { launch, pause, typeNaturally } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Users 500 Error\n");

  const { context, page } = await launch();

  // Navigate to users page
  await page.goto("http://localhost:3000/users");
  await pause(2000, "Users page loads with full user list");

  // Search for something that works
  await typeNaturally(page, '[data-testid="search"]', "alice");
  await pause(1500, "Search for 'alice' — results filter correctly");

  // Clear and search for the trigger term
  await page.fill('[data-testid="search"]', "");
  await pause(500, "Clear search field");

  await typeNaturally(page, '[data-testid="search"]', "admin");
  await pause(2000, "Search for 'admin' — 500 error! Error message shown");

  // Wait for extension to flush batched data to server
  await pause(5000, "Waiting for extension to flush data to Gasoline server");

  console.log("\n  ✓ 500 error triggered — Gasoline should have captured the network response body\n");

  await context.close();
}
