package caching

import (
	"context"
	"encoding/json"
)

// cacheDataProvider provides an interface for getting cache data.
type cacheDataProvider interface {
	GetCacheData(ctx context.Context) (map[string]string, error)
}

// toJSON creates a json string from an object.
func toJSON(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
