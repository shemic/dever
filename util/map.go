package util

func CloneMap(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func CloneMapSlice(items []map[string]any) []map[string]any {
	if items == nil {
		return nil
	}
	if len(items) == 0 {
		return []map[string]any{}
	}

	cloned := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		cloned = append(cloned, CloneMap(item))
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}
