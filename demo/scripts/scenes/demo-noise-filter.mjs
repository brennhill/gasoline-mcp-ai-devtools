// Demo: Noise Filter — Shows configure/noise_rule filtering repetitive logs
// Generates noisy heartbeat logs, then shows how to dismiss them

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Noise Filter (Smart Log Filtering)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to logs page
  await page.goto("http://localhost:3000/logs");
  await pause(3000, "Logs page streaming — mixed signal and noise");

  // Step 2: Flood heartbeat messages (noise)
  await page.click("text=Flood Heartbeats");
  await pause(1000);
  await page.click("text=Flood Heartbeats");
  await pause(1000);
  await page.click("text=Flood Heartbeats");
  await pause(2000, "30 heartbeat messages generated — this is the noise");

  // Step 3: Trigger actual errors (signal)
  await page.click("text=Trigger Error Burst");
  await pause(2000, "3 CRITICAL errors — this is the signal we care about");

  // Step 4: Wait for flush
  await pause(3000, "All 33+ log entries captured...");

  console.log("\n  \u2713 Noisy logs captured — ready to demonstrate filtering\n");
  console.log("  Without filtering: observe({what:'logs'}) returns 33+ entries");
  console.log("  ");
  console.log("  To filter noise:");
  console.log("  1. configure({action:'noise_rule', noise_action:'add', rules:[{");
  console.log("       category:'console', matchSpec:{messageRegex:'Heartbeat.*ping'}");
  console.log("     }]})");
  console.log("  2. Now observe({what:'logs'}) only shows the 3 CRITICAL errors");
  console.log("  ");
  console.log("  Or auto-detect: configure({action:'noise_rule', noise_action:'auto_detect'})");
  console.log("  Gasoline identifies repetitive patterns automatically!\n");

  await context.close();
}
