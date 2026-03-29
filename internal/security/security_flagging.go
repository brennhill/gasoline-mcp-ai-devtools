// Purpose: Flags suspicious origins, ports, and supply-chain indicators from captured network activity.
// Why: Surfaces high-signal threat indicators that are otherwise easy to miss in raw telemetry.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// analyzeNetworkSecurity runs the full set of origin/resource checks for one network entry.
func analyzeNetworkSecurity(entry capture.NetworkWaterfallEntry, pageURL string) []capture.SecurityFlag {
	origin := util.ExtractOrigin(entry.URL)
	if origin == "" {
		return nil
	}

	var flags []capture.SecurityFlag

	// Run all detection algorithms
	checks := []func(string) *capture.SecurityFlag{
		checkSuspiciousTLD,
		checkNonStandardPort,
		checkIPAddressOrigin,
		checkTyposquatting,
	}

	for _, check := range checks {
		if flag := check(origin); flag != nil {
			flag.Resource = entry.URL
			flag.PageURL = pageURL
			flags = append(flags, *flag)
		}
	}

	// Mixed content check requires both entry and page URL
	if flag := checkMixedContent(entry, pageURL); flag != nil {
		flags = append(flags, *flag)
	}

	return flags
}
