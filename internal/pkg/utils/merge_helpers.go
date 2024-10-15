package utils

// MergeMaps merges two maps, with keys from the second map overriding keys in the first map.
func MergeMaps[K comparable, V any](map1, map2 map[K]V) map[K]V {
	result := make(map[K]V)

	// Copy map1 to result
	for k, v := range map1 {
		result[k] = v
	}

	// Merge map2 into result
	for k, v := range map2 {
		result[k] = v
	}

	return result
}
