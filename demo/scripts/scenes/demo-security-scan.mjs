// Demo: Security Scan — Shows security_audit detecting PII and secrets
// Fills checkout form with sensitive data, submits, shows findings

import { launch, pause, typeNaturally } from "../utils/setup.mjs";

export async function run() {
  console.log("\n\u{1F3AC} Demo: Security Scan (PII & Secret Detection)\n");

  const { context, page } = await launch();

  // Step 1: Navigate to checkout (has PII form + insecure submission)
  await page.goto("http://localhost:3000/checkout");
  await pause(1500, "Checkout page loaded — form with PII fields");

  // Step 2: Fill form with sensitive data
  await typeNaturally(page, '[data-testid="name"]', "Jane Smith");
  await typeNaturally(page, '[data-testid="email"]', "jane.smith@company.com");
  await typeNaturally(page, '[data-testid="phone"]', "+1-555-867-5309");
  await typeNaturally(page, '[data-testid="ssn"]', "123-45-6789");
  await typeNaturally(page, '[data-testid="cardNumber"]', "4532015112830366");
  await typeNaturally(page, '[data-testid="cardExpiry"]', "12/28");
  await typeNaturally(page, '[data-testid="cardCvc"]', "456");
  await pause(1000, "Sensitive PII entered: SSN, card number, phone");

  // Step 3: Submit the form (sends PII in request body)
  await page.click('[data-testid="submit-payment"]');
  await pause(3000, "Payment submitted — raw PII sent in POST body");

  // Step 4: Visit analytics page (PII leakage to third parties)
  await page.goto("http://localhost:3000/analytics");
  await pause(3000, "Analytics page loads — PII sent to third-party trackers");

  // Step 5: Wait for flush
  await pause(3000, "Extension captures all network requests with bodies...");

  console.log("\n  \u2713 Security issues ready for detection\n");
  console.log("  Try: security_audit — Gasoline will find:");
  console.log("    - Card number (4532015112830366) in POST body");
  console.log("    - SSN (123-45-6789) in POST body");
  console.log("    - PII fields (email, phone) sent to third-party origins");
  console.log("    - Missing security headers");
  console.log("    - Console logging of sensitive data\n");

  await context.close();
}
