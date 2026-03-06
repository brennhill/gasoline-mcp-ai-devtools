  // --- PARTIAL: Overlay & Stability Action Handlers ---
  // #502: Extracted to self-contained modules for < 800 LOC file sizes.
  //   - dom-primitives-overlay.ts: dismiss_top_overlay, auto_dismiss_overlays
  //   - dom-primitives-stability.ts: wait_for_stable, action_diff
  // These actions are dispatched directly by dom-dispatch.ts and no longer go through domPrimitive.
    }
  }
