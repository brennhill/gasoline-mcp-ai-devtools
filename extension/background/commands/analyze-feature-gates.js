// analyze-feature-gates.ts — Feature gate detection command handler (#345).
import { analyzeFeatureGates } from '../analyze-feature-gates.js';
import { registerCommand } from './registry.js';
// =============================================================================
// FEATURE GATES DETECTION (#345)
// =============================================================================
registerCommand('feature_gates', async (ctx) => {
    try {
        const results = await chrome.scripting.executeScript({
            target: { tabId: ctx.tabId },
            world: 'MAIN',
            func: analyzeFeatureGates
        });
        const result = results?.[0]?.result;
        if (!result) {
            ctx.sendResult({
                error: 'feature_gates_failed',
                message: 'Feature gates scan returned no result'
            });
            return;
        }
        if (ctx.query.correlation_id) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', result);
        }
        else {
            ctx.sendResult(result);
        }
    }
    catch (err) {
        const message = err.message || 'Feature gates scan failed';
        if (ctx.query.correlation_id) {
            ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, message);
        }
        else {
            ctx.sendResult({
                error: 'feature_gates_failed',
                message
            });
        }
    }
});
//# sourceMappingURL=analyze-feature-gates.js.map