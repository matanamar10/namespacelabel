package utils

// MergeMaps merges two maps into one, with values from newMap taking precedence.
func MergeMaps(existingMap, newMap map[string]string) map[string]string {
	for key, value := range newMap {
		existingMap[key] = value
	}
	return existingMap
}

// ContainsString checks if a string is in a slice.
func ContainsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// RemoveString removes a string from a slice.
func RemoveString(slice []string, str string) []string {
	var result []string
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}
