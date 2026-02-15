import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const scriptPath = path.join(__dirname, 'install-upgrade-regression.mjs');

test('upgrade regression script validates health service identity', () => {
  const source = fs.readFileSync(scriptPath, 'utf8');
  assert.match(source, /service-name/, 'expected service-name validation in health checks');
  assert.match(
    source,
    /gasoline/i,
    'expected service identity check to enforce gasoline'
  );
});
