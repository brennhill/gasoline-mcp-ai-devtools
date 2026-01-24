// Demo: Console Logs — Shows observe tool capturing all log levels
// Navigates to /logs, triggers different log levels, shows the AI can see them all

import { launch, pause, openPopup } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Console Logs (All Log Levels)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to logs page
  await page.goto("http://localhost:3000/logs");
  await pause(2000, "Logs page loaded — streaming console output begins");

  // Step 2: Wait for natural log accumulation
  await pause(6000, "Logs streaming at all levels (log, info, warn, debug)...");

  // Step 3: Trigger error burst
  await page.click("text=Trigger Error Burst");
  await pause(2000, "3 CRITICAL errors fired to console.error");

  // Step 4: Trigger noise flood
  await page.click("text=Flood Heartbeats");
  await pause(2000, "10 identical heartbeat messages — perfect for noise filtering");

  // Step 5: Wait for extension flush
  await pause(3000, "Extension batches and sends all logs to server...");

  // Step 6: Show popup
  await openPopup(context);
  await pause(3000, "Popup shows captured errors. AI can now call observe({what:'logs'})");

  console.log("\n  \u2713 All log levels captured — errors, warnings, info, and debug\n");
  console.log("  Try: observe({what:'logs'}) to see everything");
  console.log("  Try: observe({what:'errors'}) to filter to just errors\n");

  await context.close();
}
