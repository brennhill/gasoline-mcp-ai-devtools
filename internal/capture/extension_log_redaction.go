package capture

import "encoding/json"

// redactExtensionLog scrubs sensitive data from extension log fields before storage.
func (c *Capture) redactExtensionLog(log ExtensionLog) ExtensionLog {
	if c.logRedactor == nil {
		return log
	}

	log.Message = c.logRedactor.Redact(log.Message)
	log.Source = c.logRedactor.Redact(log.Source)
	log.Category = c.logRedactor.Redact(log.Category)
	if len(log.Data) > 0 {
		log.Data = c.redactExtensionLogData(log.Data)
	}
	return log
}

func (c *Capture) redactExtensionLogData(data json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return json.RawMessage(c.logRedactor.Redact(string(data)))
	}

	redacted := redactJSONValue(value, c.logRedactor.Redact)
	output, err := json.Marshal(redacted)
	if err != nil {
		return json.RawMessage(c.logRedactor.Redact(string(data)))
	}
	return output
}

func redactJSONValue(value any, redactFn func(string) string) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			typed[key] = redactJSONValue(child, redactFn)
		}
		return typed
	case []any:
		for i, child := range typed {
			typed[i] = redactJSONValue(child, redactFn)
		}
		return typed
	case string:
		return redactFn(typed)
	default:
		return value
	}
}
