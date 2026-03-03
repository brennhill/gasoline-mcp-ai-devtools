/**
 * Purpose: Command handlers for the observe MCP tool (screenshot capture, network waterfall, page info, tab listing).
 * Docs: docs/features/feature/observe/index.md
 */
export declare function screenshotExpandContainers(): {
    expanded: number;
    content_height_hint: number;
};
export declare function screenshotRestoreContainers(): void;
export declare function computeFullPageCaptureDimensions(contentWidth: number, contentHeight: number, hintedHeight: number): {
    width: number;
    height: number;
};
//# sourceMappingURL=observe.d.ts.map
