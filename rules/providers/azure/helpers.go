package azure

func strAttr(attrs map[string]interface{}, key string) string {
	if attrs == nil {
		return ""
	}
	v, ok := attrs[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func boolAttr(attrs map[string]interface{}, key string) bool {
	if attrs == nil {
		return false
	}
	v, ok := attrs[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func intAttr(attrs map[string]interface{}, key string) int {
	if attrs == nil {
		return 0
	}
	v, ok := attrs[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return nil
	}
	s, _ := v.([]interface{})
	return s
}

func firstBlock(attrs map[string]interface{}, key string) map[string]interface{} {
	items := toSlice(attrs[key])
	if len(items) == 0 {
		return nil
	}
	m, _ := items[0].(map[string]interface{})
	return m
}
