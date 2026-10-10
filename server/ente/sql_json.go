package ente

import (
	"encoding/json"
	"fmt"
)

// PostgreSQL returns JSON as bytes; libSQL returns its JSON text as a string.
func scanJSON(value any, target any) error {
	var data []byte
	switch value := value.(type) {
	case []byte:
		data = value
	case string:
		data = []byte(value)
	default:
		return fmt.Errorf("cannot scan %T as JSON", value)
	}
	return json.Unmarshal(data, target)
}
