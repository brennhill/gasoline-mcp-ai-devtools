/**
 * Purpose: Popup "Update now" button — wires self-update endpoint + reload-extension prompt.
 * Why: Lets users one-click upgrade the daemon from inside the extension.
 * Docs: docs/features/feature/self-update/index.md
 */
interface HealthResponse {
    version?: string;
    available_version?: string;
}
/**
 * Render the update-available banner based on latest health. No-op if no
 * upgrade is offered by the daemon.
 */
export declare function renderUpdateAvailableBanner(health: HealthResponse): Promise<void>;
export {};
//# sourceMappingURL=update-button.d.ts.map