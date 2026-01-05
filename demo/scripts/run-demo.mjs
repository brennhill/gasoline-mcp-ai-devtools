#!/usr/bin/env node

// Demo runner — runs individual scenes or all scenes sequentially
// Usage:
//   node scripts/run-demo.mjs users-500
//   node scripts/run-demo.mjs --all

import { clearGasoline } from "./utils/setup.mjs";

const scenes = {
  "dashboard-cls": () => import("./scenes/dashboard-cls.mjs"),
  "users-500": () => import("./scenes/users-500.mjs"),
  "notifications-ws": () => import("./scenes/notifications-ws.mjs"),
  "settings-rejection": () => import("./scenes/settings-rejection.mjs"),
  "reports-undefined": () => import("./scenes/reports-undefined.mjs"),
  "billing-a11y": () => import("./scenes/billing-a11y.mjs"),
  "activity-payload": () => import("./scenes/activity-payload.mjs"),
  "demo-zero-config": () => import("./scenes/demo-zero-config.mjs"),
  "demo-websocket-toggle": () => import("./scenes/demo-websocket-toggle.mjs"),
  "demo-network-bodies": () => import("./scenes/demo-network-bodies.mjs"),
  "demo-full-observability": () => import("./scenes/demo-full-observability.mjs"),
};

async function main() {
  const args = process.argv.slice(2);

  if (args.length === 0 || args.includes("--help")) {
    console.log(`
Demo Runner — "What SaaS?" Demo Automation
============================================

Usage:
  node scripts/run-demo.mjs <scene-name>    Run a single scene
  node scripts/run-demo.mjs --all           Run all scenes sequentially

Available scenes:
${Object.keys(scenes).map((s) => `  - ${s}`).join("\n")}

Prerequisites:
  1. Start the demo app:        npm run dev
  2. Start the WS server:       node ws-server.mjs
  3. Start Gasoline:            npx gasoline
  4. Run a scene:               node scripts/run-demo.mjs users-500
`);
    process.exit(0);
  }

  if (args.includes("--all")) {
    console.log("🎬 Running ALL demo scenes\n");
    for (const [name, loader] of Object.entries(scenes)) {
      console.clear();
      await clearGasoline();
      console.log(`\n${"=".repeat(50)}`);
      console.log(`  Scene: ${name}`);
      console.log("=".repeat(50));
      try {
        const mod = await loader();
        await mod.run();
      } catch (err) {
        console.error(`  ❌ Scene "${name}" failed: ${err.message}`);
      }
      // Brief pause between scenes
      await new Promise((r) => setTimeout(r, 2000));
    }
    console.log("\n\n✅ All scenes completed!\n");
  } else {
    const sceneName = args[0];
    if (!scenes[sceneName]) {
      console.error(`Unknown scene: "${sceneName}"\nAvailable: ${Object.keys(scenes).join(", ")}`);
      process.exit(1);
    }

    console.clear();
    await clearGasoline();
    try {
      const mod = await scenes[sceneName]();
      await mod.run();
    } catch (err) {
      console.error(`❌ Scene "${sceneName}" failed:`, err);
      process.exit(1);
    }
  }
}

main();
