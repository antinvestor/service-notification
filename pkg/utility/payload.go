package utility

import (
	"fmt"
	"strconv"
)

// PayloadString returns payload[key] rendered as a string. Numbers decoded from JSON
// (float64) are formatted without an exponent so ids and codes survive intact.
func PayloadString(payload map[string]any, key string) string {
	v, ok := payload[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// PayloadInt returns payload[key] as an int, tolerating JSON float64 and numeric strings.
// The second value reports whether a usable number was present.
func PayloadInt(payload map[string]any, key string) (int, bool) {
	v, ok := payload[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case string:
		n, err := strconv.Atoi(t)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// PayloadStrings flattens every top-level payload value to a string, for status extras.
func PayloadStrings(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload))
	for k := range payload {
		out[k] = PayloadString(payload, k)
	}
	return out
}
