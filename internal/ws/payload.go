package ws

func str(p map[string]any, key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return ""
}

func strSlice(p map[string]any, key string) []string {
	values, ok := p[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intVal(p map[string]any, key string, fallback int) int {
	switch v := p[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return fallback
	}
}

func roomName(eventID, eventDateID string) string {
	return eventID + "_" + eventDateID
}

func areaName(eventID, eventDateID, areaID string) string {
	return eventID + "_" + eventDateID + "_" + areaID
}

func ptr(value string) *string { return &value }
