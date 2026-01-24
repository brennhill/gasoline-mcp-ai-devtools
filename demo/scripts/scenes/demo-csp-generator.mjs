// Demo: CSP Generator — Shows generate_csp building a Content Security Policy
// Visits analytics page which loads resources from multiple origins

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: CSP Generator (Passive Policy Generation)\n");

  const { context, page } = await launch();

  // Step 1: Visit the analytics page (loads third-party scripts)
  await page.goto("http://localhost:3000/analytics");
  await pause(3000, "Analytics page loaded — third-party scripts attempted");

  // Step 2: Navigate around to accumulate more origins
  await page.goto("http://localhost:3000/");
  await pause(1500, "Dashboard loaded (self-origin resources)");

  await page.goto("http://localhost:3000/integrations");
  await pause(1500, "Integrations loaded (API calls to self-origin)");

  await page.goto("http://localhost:3000/analytics");
  await pause(2000, "Back to analytics — more tracker requests observed");

  // Step 3: Wait for flush
  await pause(3000, "Extension captured all network requests and script loads...");

  console.log("\n  \u2713 Multiple origins observed across navigation\n");
  console.log("  Try: generate_csp — Gasoline will generate a CSP like:");
  console.log("    default-src 'self';");
  console.log("    script-src 'self' 'unsafe-inline' cdn.sketchy-analytics.example.com;");
  console.log("    connect-src 'self' analytics.tracker-cdn.example.com;");
  console.log("    img-src 'self' pixel.adnetwork.example.com;");
  console.log("    style-src 'self' 'unsafe-inline';");
  console.log("  ");
  console.log("  The CSP is built passively from observed network activity!\n");

  await context.close();
}
