// Scene: Reports — Cannot read properties of undefined (reading 'map')
// Race condition: data is null before loading completes

import { launch, pause } from "../utils/setup.mjs";

export async function run() {
  console.log("\n🎬 Scene: Reports Undefined Error\n");

  const { context, page } = await launch();

  // Navigate to reports page — may crash immediately due to race condition
  page.on("pageerror", (err) => {
    console.log(`  💥 Page error caught: ${err.message.substring(0, 80)}`);
  });

  await page.goto("http://localhost:3000/reports");
  await pause(3000, "Reports page loads — may show blank/error due to race condition");

  // Reload a few times to trigger the race condition
  for (let i = 0; i < 3; i++) {
    await page.reload();
    await pause(1500, `Reload ${i + 1} — checking for race condition...`);
  }

  console.log("\n  ✓ Race condition triggered — Gasoline should have captured the TypeError\n");

  await context.close();
}
