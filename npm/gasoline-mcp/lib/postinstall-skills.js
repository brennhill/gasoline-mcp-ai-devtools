#!/usr/bin/env node
/**
 * Postinstall hook for bundled Gasoline skills.
 * Never fails package installation.
 */

const { installBundledSkills } = require('./skills');

async function run() {
  try {
    const verbose =
      String(process.env.GASOLINE_SKILL_VERBOSE || '')
        .trim()
        .toLowerCase() === '1' ||
      String(process.env.GASOLINE_SKILL_VERBOSE || '')
        .trim()
        .toLowerCase() === 'true';

    const result = await installBundledSkills({ verbose });

    if (result.skipped) {
      if (verbose) {
        console.log(`[gasoline-mcp] skill install skipped: ${result.reason}`);
      }
      return;
    }

    const s = result.summary;
    console.log(
      `[gasoline-mcp] skills installed (${result.agents.join(', ')} / ${result.scope}): ` +
        `source=${result.source} created=${s.created} updated=${s.updated} unchanged=${s.unchanged} ` +
        `skipped=${s.skipped_user_owned} legacy_removed=${s.legacy_removed} errors=${s.errors}`
    );
    for (const warning of result.warnings || []) {
      console.warn(`[gasoline-mcp] warning: ${warning}`);
    }
  } catch (err) {
    console.warn(`[gasoline-mcp] warning: skill postinstall failed: ${err.message}`);
  }
}

run();
