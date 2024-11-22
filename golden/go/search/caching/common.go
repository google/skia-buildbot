package caching

import (
	"encoding/json"

	"go.skia.org/infra/golden/go/sql/schema"
)

// SearchCacheData provides a struct to hold data for the entry in by blame cache.
type SearchCacheData struct {
	TraceID    schema.TraceID     `json:"traceID"`
	GroupingID schema.GroupingID  `json:"groupingID"`
	Digest     schema.DigestBytes `json:"digest"`
}

// toJSON creates a json string from an object.
func toJSON(obj interface{}) (string, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
