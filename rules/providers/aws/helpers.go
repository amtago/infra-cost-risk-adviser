package aws

import "encoding/json"

// jsonUnmarshal is an alias so tier files can call it without importing encoding/json directly.
var jsonUnmarshal = json.Unmarshal

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

// toSlice coerces an interface{} to []interface{}, returning nil if not possible.
func toSlice(v interface{}) []interface{} {
	s, _ := v.([]interface{})
	return s
}

// hasCIDR reports whether a security group rule map contains the given CIDR in cidr_blocks or ipv6_cidr_blocks.
func hasCIDR(rule map[string]interface{}, cidr string) bool {
	for _, key := range []string{"cidr_blocks", "ipv6_cidr_blocks"} {
		for _, c := range toSlice(rule[key]) {
			if s, ok := c.(string); ok && s == cidr {
				return true
			}
		}
	}
	return false
}

// sliceContains reports whether s contains target.
func sliceContains(s []string, target string) bool {
	for _, v := range s {
		if v == target {
			return true
		}
	}
	return false
}

// toStringSlice coerces an interface{} that is either a string or []interface{} into []string.
func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case string:
		return []string{val}
	case []interface{}:
		var out []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
