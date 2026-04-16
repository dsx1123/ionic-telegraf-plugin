package nicctl

import (
	"encoding/json"
	"fmt"
	"strings"
)

// sanitizeKey replaces characters that are problematic in metric field names.
func sanitizeKey(key string) string {
	r := strings.NewReplacer(" ", "_", ",", "_", ".", "_", "=", "_")
	return r.Replace(key)
}

// FlattenJSON parses raw JSON bytes and returns a flat map of field names to values.
// Objects are recursively walked with keys joined by "_".
// Arrays produce index-suffixed keys (key_0, key_1, ...).
// Numbers without fractional parts are narrowed to int64.
// Nulls are skipped.
func FlattenJSON(data []byte) (map[string]interface{}, error) {
	var parsed interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	result := make(map[string]interface{})
	flatten("", parsed, result)
	return result, nil
}

func flatten(prefix string, value interface{}, result map[string]interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, val := range v {
			newKey := sanitizeKey(key)
			if prefix != "" {
				newKey = prefix + "_" + newKey
			}
			flatten(newKey, val, result)
		}
	case []interface{}:
		for i, val := range v {
			newKey := fmt.Sprintf("%s_%d", prefix, i)
			flatten(newKey, val, result)
		}
	case float64:
		if v == float64(int64(v)) {
			result[prefix] = int64(v)
		} else {
			result[prefix] = v
		}
	case string:
		result[prefix] = v
	case bool:
		result[prefix] = v
	case nil:
		// skip nulls
	}
}
