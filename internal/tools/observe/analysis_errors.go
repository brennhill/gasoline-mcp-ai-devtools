// Purpose: Groups captured error logs into stable clusters for high-signal observe output.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

type errorCluster struct {
	message    string
	level      string
	count      int
	firstSeen  string
	lastSeen   string
	urls       map[string]bool
	stackTrace string
}

// AnalyzeErrors clusters error entries by message for pattern detection.
func AnalyzeErrors(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	entries, _ := deps.GetLogEntries()
	clusters := buildErrorClusters(entries)
	result := clustersToResponse(clusters)

	return mcp.Succeed(req, "Error clusters", map[string]any{
		"clusters":    result,
		"total_count": len(result),
		"metadata":    BuildResponseMetadata(deps.GetCapture(), time.Now()),
	})
}

func buildErrorClusters(entries []map[string]any) map[string]*errorCluster {
	clusters := make(map[string]*errorCluster)
	for _, entry := range entries {
		level, _ := entry["level"].(string)
		if level != "error" {
			continue
		}
		msg, _ := entry["message"].(string)
		if msg == "" {
			continue
		}

		clusterKey := msg
		if len(clusterKey) > 100 {
			clusterKey = clusterKey[:100]
		}

		timestamp, _ := entry["timestamp"].(string)
		url, _ := entry["url"].(string)
		stack, _ := entry["stackTrace"].(string)

		addToCluster(clusters, clusterKey, msg, level, timestamp, url, stack)
	}
	return clusters
}

func addToCluster(clusters map[string]*errorCluster, key, msg, level, timestamp, url, stack string) {
	if cluster, exists := clusters[key]; exists {
		cluster.count++
		cluster.lastSeen = timestamp
		if url != "" {
			cluster.urls[url] = true
		}
		return
	}
	urls := make(map[string]bool)
	if url != "" {
		urls[url] = true
	}
	clusters[key] = &errorCluster{
		message:    msg,
		level:      level,
		count:      1,
		firstSeen:  timestamp,
		lastSeen:   timestamp,
		urls:       urls,
		stackTrace: stack,
	}
}

func clustersToResponse(clusters map[string]*errorCluster) []map[string]any {
	result := make([]map[string]any, 0, len(clusters))
	for _, c := range clusters {
		urlList := make([]string, 0, len(c.urls))
		for u := range c.urls {
			urlList = append(urlList, u)
		}
		result = append(result, map[string]any{
			"message":     c.message,
			"count":       c.count,
			"first_seen":  c.firstSeen,
			"last_seen":   c.lastSeen,
			"urls":        urlList,
			"stack_trace": c.stackTrace,
		})
	}
	return result
}
