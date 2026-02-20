// indexeddb.go — Handlers and helpers for IndexedDB inspection via observe tool.
package observe

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/queries"
)

const indexedDBQueryTimeout = 10 * time.Second

// GetIndexedDB returns rows from one IndexedDB object store.
func GetIndexedDB(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)}
	}
	if params.Store == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='indexeddb'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	storeData, err := getIndexedDBEntries(cap, params.Database, params.Store, params.Limit)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+err.Error(),
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	entries, _ := storeData["entries"].([]any)
	count := len(entries)
	if c, ok := toInt(storeData["count"]); ok {
		count = c
	}

	response := map[string]any{
		"database": params.Database,
		"store":    params.Store,
		"entries":  entries,
		"count":    count,
		"limit":    params.Limit,
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}
	if v, ok := storeData["object_stores"]; ok {
		response["object_stores"] = v
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("IndexedDB entries", response)}
}

func getIndexedDBListing(cap *capture.Capture) (map[string]any, error) {
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

func getIndexedDBEntries(cap *capture.Capture, database, store string, limit int) (map[string]any, error) {
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

func executeObserveScript(cap *capture.Capture, script, reason string, timeout time.Duration) (map[string]any, error) {
	params, _ := json.Marshal(map[string]any{
		"script":     script,
		"timeout_ms": int(timeout.Milliseconds()),
		"world":      "auto",
		"reason":     reason,
	})

	queryID := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "execute",
			Params: params,
		},
		timeout,
		"",
	)

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

func buildIndexedDBEntriesScript(database, store string, limit int) string {
	return fmt.Sprintf(`(() => (async () => {
  const database = %s;
  const store = %s;
  const limit = %d;
  try {
    if (typeof indexedDB === "undefined") {
      return { ok: false, error: "indexeddb_unsupported" };
    }

    const openReq = indexedDB.open(database);
    const db = await new Promise((resolve, reject) => {
      openReq.onsuccess = () => resolve(openReq.result);
      openReq.onerror = () => reject(openReq.error || new Error("indexeddb_open_failed"));
      openReq.onblocked = () => reject(new Error("indexeddb_open_blocked"));
    });

    const objectStores = Array.from(db.objectStoreNames || []);
    if (!objectStores.includes(store)) {
      db.close();
      return { ok: false, error: "store_not_found", message: "Object store not found", object_stores: objectStores };
    }

    const tx = db.transaction(store, "readonly");
    const objectStore = tx.objectStore(store);

    function serialize(value, depth = 0) {
      if (depth > 8) return "[max_depth]";
      if (value === null || value === undefined) return value;
      const t = typeof value;
      if (t === "string" || t === "number" || t === "boolean") return value;
      if (t === "bigint") return value.toString();
      if (t === "symbol") return String(value);
      if (Array.isArray(value)) return value.slice(0, 100).map((v) => serialize(v, depth + 1));
      if (value instanceof Date) return value.toISOString();
      if (ArrayBuffer.isView(value)) return Array.from(value).slice(0, 1000);
      if (value instanceof ArrayBuffer) return { byte_length: value.byteLength };
      if (t === "object") {
        const out = {};
        for (const key of Object.keys(value).slice(0, 100)) {
          try {
            out[key] = serialize(value[key], depth + 1);
          } catch {
            out[key] = "[unserializable]";
          }
        }
        return out;
      }
      return String(value);
    }

    const entries = [];
    const pushEntry = (key, value) => {
      entries.push({ key: serialize(key), value: serialize(value) });
    };

    if (typeof objectStore.getAll === "function" && typeof objectStore.getAllKeys === "function") {
      const [values, keys] = await Promise.all([
        new Promise((resolve, reject) => {
          const req = objectStore.getAll(undefined, limit);
          req.onsuccess = () => resolve(req.result || []);
          req.onerror = () => reject(req.error || new Error("indexeddb_get_all_failed"));
        }),
        new Promise((resolve, reject) => {
          const req = objectStore.getAllKeys(undefined, limit);
          req.onsuccess = () => resolve(req.result || []);
          req.onerror = () => reject(req.error || new Error("indexeddb_get_all_keys_failed"));
        })
      ]);
      for (let i = 0; i < Math.min(keys.length, values.length); i++) {
        pushEntry(keys[i], values[i]);
      }
    } else {
      await new Promise((resolve, reject) => {
        const req = objectStore.openCursor();
        req.onerror = () => reject(req.error || new Error("indexeddb_cursor_failed"));
        req.onsuccess = () => {
          const cursor = req.result;
          if (!cursor || entries.length >= limit) {
            resolve(undefined);
            return;
          }
          pushEntry(cursor.key, cursor.value);
          cursor.continue();
        };
      });
    }

    db.close();

    return {
      ok: true,
      database,
      store,
      object_stores: objectStores,
      entries,
      count: entries.length,
      limit
    };
  } catch (e) {
    return { ok: false, error: String((e && e.message) || e) };
  }
})())()`, jsStringLiteral(database), jsStringLiteral(store), limit)
}

const indexedDBListingScript = `(() => (async () => {
  try {
    if (typeof indexedDB === "undefined") {
      return { supported: false, databases: [] };
    }
    if (typeof indexedDB.databases !== "function") {
      return { supported: false, databases: [], error: "indexeddb_databases_unavailable" };
    }

    const infos = await indexedDB.databases();
    const databases = [];
    for (const info of infos || []) {
      if (!info || !info.name) continue;
      const name = String(info.name);
      const version = typeof info.version === "number" ? info.version : null;
      let objectStores = [];
      try {
        const req = indexedDB.open(name);
        const db = await new Promise((resolve, reject) => {
          req.onsuccess = () => resolve(req.result);
          req.onerror = () => reject(req.error || new Error("indexeddb_open_failed"));
          req.onblocked = () => reject(new Error("indexeddb_open_blocked"));
        });
        objectStores = Array.from(db.objectStoreNames || []);
        db.close();
      } catch {
        objectStores = [];
      }

      databases.push({
        name,
        version,
        object_stores: objectStores
      });
    }

    return { supported: true, databases };
  } catch (e) {
    return { supported: false, databases: [], error: String((e && e.message) || e) };
  }
})())()`

func jsStringLiteral(v string) string {
	b, _ := json.Marshal(v)
	return string(b)
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
