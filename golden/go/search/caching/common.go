package caching

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/golden/go/sql/schema"
)

// ByBlameData provides a struct to hold data for the entry in by blame cache.
type ByBlameData struct {
	TraceID    schema.TraceID     `json:"traceID"`
	GroupingID schema.GroupingID  `json:"groupingID"`
	Digest     schema.DigestBytes `json:"digest"`
}

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
