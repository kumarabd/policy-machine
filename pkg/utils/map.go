package utils

import "fmt"

// InterfaceToStringMap converts generic context map to string map for the service layer
func InterfaceToStringMap(context map[string]interface{}) map[string]string {
	if context == nil {
		return nil
	}

	stringMap := make(map[string]string)
	for key, value := range context {
		if str, ok := value.(string); ok {
			stringMap[key] = str
		} else {
			// Convert other types to string representation
			stringMap[key] = fmt.Sprintf("%v", value)
		}
	}

	return stringMap
}
