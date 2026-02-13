// kill-daemon.js — Kill running gasoline daemon before install/uninstall.
// Cross-platform, never fails (all errors are swallowed).
const { execSync } = require('child_process');

try {
  const cmd = process.platform === 'win32'
    ? 'taskkill /F /IM gasoline.exe 2>nul'
    : 'pkill -9 -x gasoline 2>/dev/null';
  execSync(cmd, { stdio: 'ignore', shell: true, timeout: 5000 });
} catch (_) {
  // Process not running or kill failed — either way, fine.
}
