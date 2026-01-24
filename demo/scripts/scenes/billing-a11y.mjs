// Scene: Billing — Accessibility violations
// Missing labels, broken tab order, no error announcements
// Gasoline's a11y audit tool catches these

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Billing Accessibility Violations\n");

  const { context, page } = await launch();

  // Navigate to billing page
  await page.goto("http://localhost:3000/billing");
  await pause(2000, "Billing page loads — looks fine visually");

  // Try tabbing through the form to show broken tab order
  await page.keyboard.press("Tab");
  await pause(500, "Tab 1 — focus jumps to wrong field (broken tabIndex)");
  await page.keyboard.press("Tab");
  await pause(500, "Tab 2 — order is wrong");
  await page.keyboard.press("Tab");
  await pause(500, "Tab 3 — still wrong order");

  await pause(2000, "Form looks correct visually but has WCAG violations");

  console.log("\n  ✓ A11y violations present — Gasoline's run_accessibility_audit will find them\n");

  await context.close();
}
