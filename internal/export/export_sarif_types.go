package export

// version is set at build time via -ldflags "-X ...internal/export.version=..."
// Fallback used for `go run` (no ldflags).
var version = "dev"

// SARIF 2.1.0 specification constants
const (
	sarifSpecVersion = "2.1.0"
	sarifSchemaURL   = "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/main/sarif-2.1/schema/sarif-schema-2.1.0.json"
)

// SARIFLog is the top-level SARIF 2.1.0 object.
type SARIFLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []SARIFRun `json:"runs"`
}

// SARIFRun represents a single analysis run.
type SARIFRun struct {
	Tool    SARIFTool     `json:"tool"`
	Results []SARIFResult `json:"results"`
}

// SARIFTool describes the analysis tool.
type SARIFTool struct {
	Driver SARIFDriver `json:"driver"`
}

// SARIFDriver describes the tool driver (primary component).
type SARIFDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version"`
	InformationURI string      `json:"informationUri"` // SPEC:SARIF
	Rules          []SARIFRule `json:"rules"`
}

// SARIFRule describes a single analysis rule.
type SARIFRule struct {
	ID               string               `json:"id"`
	ShortDescription SARIFMessage         `json:"shortDescription"` // SPEC:SARIF
	FullDescription  SARIFMessage         `json:"fullDescription"`  // SPEC:SARIF
	HelpURI          string               `json:"helpUri"`          // SPEC:SARIF
	Properties       *SARIFRuleProperties `json:"properties,omitempty"`
}

// SARIFRuleProperties holds additional rule metadata.
type SARIFRuleProperties struct {
	Tags []string `json:"tags,omitempty"`
}

// SARIFResult represents a single analysis finding.
type SARIFResult struct {
	RuleID    string          `json:"ruleId"`    // SPEC:SARIF
	RuleIndex int             `json:"ruleIndex"` // SPEC:SARIF
	Level     string          `json:"level"`
	Message   SARIFMessage    `json:"message"`
	Locations []SARIFLocation `json:"locations"`
}

// SARIFMessage is a simple text message.
type SARIFMessage struct {
	Text string `json:"text"`
}

// SARIFLocation represents a finding location.
type SARIFLocation struct {
	PhysicalLocation SARIFPhysicalLocation `json:"physicalLocation"` // SPEC:SARIF
}

// SARIFPhysicalLocation describes the physical location of a finding.
type SARIFPhysicalLocation struct {
	ArtifactLocation SARIFArtifactLocation `json:"artifactLocation"` // SPEC:SARIF
	Region           SARIFRegion           `json:"region"`
}

// SARIFArtifactLocation identifies the artifact (file, DOM element, etc.).
type SARIFArtifactLocation struct {
	URI       string `json:"uri"`
	URIBaseID string `json:"uriBaseId,omitempty"` // SPEC:SARIF
}

// SARIFRegion describes a region within an artifact.
type SARIFRegion struct {
	Snippet SARIFSnippet `json:"snippet"`
}

// SARIFSnippet contains a text snippet of the region.
type SARIFSnippet struct {
	Text string `json:"text"`
}

// SARIFExportOptions controls the export behavior.
type SARIFExportOptions struct {
	Scope         string `json:"scope"`
	IncludePasses bool   `json:"include_passes"`
	SaveTo        string `json:"save_to"`
}

type axeResult struct {
	Violations   []axeViolation `json:"violations"`
	Passes       []axeViolation `json:"passes"`
	Incomplete   []axeViolation `json:"incomplete"`
	Inapplicable []axeViolation `json:"inapplicable"`
}

type axeViolation struct {
	ID          string    `json:"id"`
	Impact      string    `json:"impact"`
	Description string    `json:"description"`
	Help        string    `json:"help"`
	HelpURL     string    `json:"helpUrl"` // SPEC:axe-core
	Tags        []string  `json:"tags"`
	Nodes       []axeNode `json:"nodes"`
}

type axeNode struct {
	HTML   string   `json:"html"`
	Target []string `json:"target"`
	Impact string   `json:"impact"`
}
