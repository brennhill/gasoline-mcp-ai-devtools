export type QueryParamsObject = Record<string, unknown>;
export type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab';
export interface TargetResolution {
    tabId: number;
    url: string;
    source: TargetResolutionSource;
    requestedTabId?: number;
    trackedTabId?: number | null;
    trackedTabUrl?: string | null;
    useActiveTab: boolean;
}
type PierceShadowInput = boolean | 'auto';
export declare function parsePierceShadowInput(value: unknown): {
    value?: PierceShadowInput;
    error?: string;
};
export declare function hasActiveDebugIntent(target: TargetResolution | undefined): boolean;
export declare function resolveDOMQueryParams(params: QueryParamsObject, target: TargetResolution | undefined): {
    params?: QueryParamsObject;
    error?: string;
};
export {};
//# sourceMappingURL=pierce-shadow.d.ts.map