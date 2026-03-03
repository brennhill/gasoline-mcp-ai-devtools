// Purpose: Encapsulates extension log ring-buffer operations behind a focused store API.
// Why: Reduces Capture god-object surface by moving append/eviction/copy/clear logic into ExtensionLogBuffer.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

// append adds one extension log entry and applies amortized eviction.
func (b *ExtensionLogBuffer) append(log ExtensionLog) {
	b.logs = append(b.logs, log)
	evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
	if len(b.logs) <= evictionThreshold {
		return
	}

	kept := make([]ExtensionLog, MaxExtensionLogs)
	copy(kept, b.logs[len(b.logs)-MaxExtensionLogs:])
	b.logs = kept
}

// snapshot returns a detached copy of the buffer contents.
func (b *ExtensionLogBuffer) snapshot() []ExtensionLog {
	out := make([]ExtensionLog, len(b.logs))
	copy(out, b.logs)
	return out
}

// clear removes all buffered logs and returns removed count.
func (b *ExtensionLogBuffer) clear() int {
	count := len(b.logs)
	b.logs = make([]ExtensionLog, 0)
	return count
}
