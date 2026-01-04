// Scene: Dashboard CLS (Cumulative Layout Shift)
// The chart loads 2s late, causing a visible layout jump

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Dashboard CLS\n");

  const { context, page } = await launch();

  // Navigate to dashboard
  await page.goto("http://localhost:3000");
  await pause(1500, "Dashboard loads — stats visible, chart area empty");

  // Wait for the CLS bug to trigger (chart loads after 2s)
  await pause(2500, "Chart appears — layout shifts down! CLS triggered");

  // Let viewer see the completed state
  await pause(2000, "Viewer sees the completed dashboard with chart");

  console.log("\n  ✓ CLS bug triggered — Gasoline should have captured performance data\n");

  await context.close();
}
