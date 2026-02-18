package helpers

// ToStringMap should only be used for labels/annotations. It doesn't support converting inner maps.
func ToStringMap(in map[string]any) map[string]string {
	out := make(map[string]string)

	for k, v := range in {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}

	return out
}

// MergeLabels should only be used for labels/annotations. It doesn't support merging inner maps.
func MergeLabels(first, second map[string]any) map[string]any {
	result := make(map[string]any, len(first)+len(second))

	for k, v := range first {
		result[k] = v
	}

	for k, v := range second {
		result[k] = v
	}

	return result
}
