// Demo: DOM Query — Shows query_dom tool selecting page elements
// Navigates to /checkout with its rich form structure, demonstrates querying

import { launch, pause, typeNaturally } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: DOM Query (Live Element Selection)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to checkout page (rich DOM structure)
  await page.goto("http://localhost:3000/checkout");
  await pause(2000, "Checkout page loaded — complex form with multiple sections");

  // Step 2: Fill in some form data (creates interesting DOM state)
  await typeNaturally(page, '[data-testid="name"]', "John Doe");
  await pause(500);
  await typeNaturally(page, '[data-testid="email"]', "john@example.com");
  await pause(500);
  await typeNaturally(page, '[data-testid="cardNumber"]', "4242424242424242");
  await pause(1000, "Form filled — DOM now has values to query");

  // Step 3: Wait for extension to capture actions
  await pause(3000, "Extension captures user actions (typing, focus events)...");

  console.log("\n  \u2713 Rich DOM structure ready for querying\n");
  console.log("  Try: query_dom({selector: 'input[data-testid]'}) — finds all form inputs");
  console.log("  Try: query_dom({selector: '.bg-slate-800'}) — finds card sections");
  console.log("  Try: query_dom({selector: '[data-testid=\"cardNumber\"]'}) — finds card input with value\n");

  await context.close();
}
