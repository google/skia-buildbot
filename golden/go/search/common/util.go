package common

import "encoding/json"

// toJSON creates a json string from an object.
func ToJSON(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
