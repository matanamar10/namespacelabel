package utils

// MergeMaps merges two maps into one, with values from newMap taking precedence.
func MergeMaps(existingMap, newMap map[string]string) map[string]string {
	for key, value := range newMap {
		existingMap[key] = value
	}
	return existingMap
}
