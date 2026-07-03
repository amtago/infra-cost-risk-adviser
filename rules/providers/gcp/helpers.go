package gcp

// boolAttr returns the bool value of a raw attribute, defaulting to false.
func boolAttr(raw map[string]interface{}, key string) bool {
	v, ok := raw[key]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// strAttr returns the string value of a raw attribute, defaulting to "".
func strAttr(raw map[string]interface{}, key string) string {
	v, ok := raw[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// intAttr returns the int value of a raw attribute (JSON numbers are float64).
func intAttr(raw map[string]interface{}, key string) int {
	v, ok := raw[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

// toSlice coerces an interface{} to []interface{}.
func toSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

// firstBlock returns the first element of a repeated block (Terraform encodes these as []interface{}).
func firstBlock(raw map[string]interface{}, key string) map[string]interface{} {
	items := toSlice(raw[key])
	if len(items) == 0 {
		return nil
	}
	m, _ := items[0].(map[string]interface{})
	return m
}
