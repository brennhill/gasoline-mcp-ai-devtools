/**
 * Purpose: Recording subsystem barrel — screen recording lifecycle, capture, and badge management.
 * Why: Groups all recording-related functionality into a cohesive module.
 */

// Core recording lifecycle
export { isRecording, startRecording, stopRecording } from './recording.js'

// Capture helpers
export { ensureOffscreenDocument, getStreamIdWithRecovery, requestRecordingGesture } from './capture.js'

// Listeners
export { installRecordingListeners } from './listeners.js'

// Utils
export {
  buildRecordingToastLabel,
  buildScreenRecordingSlug,
  startRecordingBadgeTimer,
  stopRecordingBadgeTimer
} from './utils.js'

// Badge (deprecated re-exports)
export { startRecordingBadgeTimer as badgeStart, stopRecordingBadgeTimer as badgeStop } from './badge.js'
