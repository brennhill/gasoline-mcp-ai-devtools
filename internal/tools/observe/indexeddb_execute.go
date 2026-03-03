// Purpose: Executes IndexedDB listing and entry queries via extension script dispatch.
// Why: Separates query execution and result parsing from script generation.
package observe

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

func getIndexedDBListing(cap *capture.Store) (map[string]any, error) {
	data, err := executeObserveScript(cap, indexedDBListingScript, "observe_storage_indexeddb", indexedDBQueryTimeout)
	if err != nil {
		return nil, err
	}

	if _, ok := data["supported"]; !ok {
		data["supported"] = true
	}
	if _, ok := data["databases"]; !ok {
		data["databases"] = []any{}
	}

	if dbs, ok := data["databases"].([]any); ok {
		sort.SliceStable(dbs, func(i, j int) bool {
			left, _ := dbs[i].(map[string]any)
			right, _ := dbs[j].(map[string]any)
			leftName, _ := left["name"].(string)
			rightName, _ := right["name"].(string)
			return leftName < rightName
		})
		data["databases"] = dbs
	}

	return data, nil
}

func getIndexedDBEntries(cap *capture.Store, database, store string, limit int) (map[string]any, error) {
	script := buildIndexedDBEntriesScript(database, store, limit)
	data, err := executeObserveScript(cap, script, "observe_indexeddb_entries", indexedDBQueryTimeout)
	if err != nil {
		return nil, err
	}

	if ok, hasOK := data["ok"].(bool); hasOK && !ok {
		return nil, errors.New(executeResultErrorMessage(data))
	}

	if _, ok := data["entries"]; !ok {
		data["entries"] = []any{}
	}
	if _, ok := data["count"]; !ok {
		if entries, ok := data["entries"].([]any); ok {
			data["count"] = len(entries)
		} else {
			data["count"] = 0
		}
	}
	if _, ok := data["database"]; !ok {
		data["database"] = database
	}
	if _, ok := data["store"]; !ok {
		data["store"] = store
	}

	return data, nil
}

func executeObserveScript(cap *capture.Store, script, reason string, timeout time.Duration) (map[string]any, error) {
	params, _ := json.Marshal(map[string]any{
		"script":     script,
		"timeout_ms": int(timeout.Milliseconds()),
		"world":      "auto",
		"reason":     reason,
	})

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "execute",
			Params: params,
		},
		timeout,
		"",
	)
	if qerr != nil {
		return nil, qerr
	}

	result, err := cap.WaitForResult(queryID, timeout)
	if err != nil {
		return nil, err
	}

	var payload map[string]any
	if err := json.Unmarshal(result, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse execute result: %w", err)
	}

	if successRaw, hasSuccess := payload["success"]; hasSuccess {
		success, _ := successRaw.(bool)
		if !success {
			return nil, errors.New(executeResultErrorMessage(payload))
		}
		if rawResult, ok := payload["result"].(map[string]any); ok {
			return rawResult, nil
		}
		return map[string]any{"value": payload["result"]}, nil
	}

	if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
		return nil, errors.New(errMsg)
	}

	return payload, nil
}

func executeResultErrorMessage(payload map[string]any) string {
	if errMsg, ok := payload["error"].(string); ok && errMsg != "" {
		return errMsg
	}
	if msg, ok := payload["message"].(string); ok && msg != "" {
		return msg
	}
	if result, ok := payload["result"].(map[string]any); ok {
		if errMsg, ok := result["error"].(string); ok && errMsg != "" {
			return errMsg
		}
		if msg, ok := result["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return "extension execution failed"
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
