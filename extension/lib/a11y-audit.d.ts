/**
 * Purpose: Accessibility auditing via axe-core -- loads axe-core dynamically, runs audits with timeout, and formats results.
 * Docs: docs/features/feature/query-dom/index.md
 */
interface AxeAuditParams {
    scope?: string;
    tags?: string[];
    include_passes?: boolean;
}
interface FormattedAxeNode {
    selector: string;
    html: string;
    failureSummary?: string;
}
interface FormattedAxeViolation {
    id: string;
    impact?: string;
    description: string;
    helpUrl: string;
    wcag?: string[];
    nodes: FormattedAxeNode[];
    nodeCount?: number;
}
interface FormattedAxeResults {
    violations: FormattedAxeViolation[];
    passes?: FormattedAxeViolation[];
    incomplete?: FormattedAxeViolation[];
    inapplicable?: FormattedAxeViolation[];
    summary: {
        violations: number;
        passes: number;
        incomplete: number;
        inapplicable: number;
    };
    partial?: boolean;
    error?: string;
}
interface AxeNode {
    target: string[] | string;
    html?: string;
    failureSummary?: string;
}
interface AxeViolation {
    id: string;
    impact?: string;
    description: string;
    helpUrl: string;
    tags?: string[];
    nodes?: AxeNode[];
}
interface AxeResults {
    violations?: AxeViolation[];
    passes?: AxeViolation[];
    incomplete?: AxeViolation[];
    inapplicable?: AxeViolation[];
}
interface AxeRunConfig {
    runOnly?: string[];
    resultTypes?: string[];
}
declare global {
    interface Window {
        axe?: {
            run(context: Element | Document | {
                include: string[];
            }, config?: AxeRunConfig): Promise<AxeResults>;
        };
    }
}
/**
 * Run an accessibility audit using axe-core
 */
export declare function runAxeAudit(params: AxeAuditParams): Promise<FormattedAxeResults>;
/**
 * Run axe audit with a timeout.
 * Issue #276: Returns partial results on timeout or conflict instead of throwing.
 */
export declare function runAxeAuditWithTimeout(params: AxeAuditParams, timeoutMs?: number): Promise<FormattedAxeResults>;
/**
 * Format axe-core results into a compact representation
 */
export declare function formatAxeResults(axeResult: AxeResults): FormattedAxeResults;
export {};
//# sourceMappingURL=a11y-audit.d.ts.map