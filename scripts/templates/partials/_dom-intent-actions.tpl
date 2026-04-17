  // --- PARTIAL: Intent Action Resolution ---
  // #502: Extracted to self-contained modules for < 800 LOC file sizes.
  //   - dom-primitives-intent.ts: open_composer, submit_active_composer, confirm_top_dialog
  //   - dom-primitives-overlay.ts: dismiss_top_overlay, auto_dismiss_overlays
  //   - dom-primitives-stability.ts: wait_for_stable, action_diff
  // These actions are dispatched directly by dom-dispatch.ts and no longer go through domPrimitive.
