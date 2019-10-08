package diffstore

import (
	"encoding/json"

	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

// MetricMapCodec implements the util.LRUCodec interface by serializing and
// deserializing generic diff result structs, instances of map[string]interface{}
type MetricMapCodec struct{}

// See util.LRUCodec interface
func (m MetricMapCodec) Encode(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// See util.LRUCodec interface
func (m MetricMapCodec) Decode(byteData []byte) (interface{}, error) {
	dm := map[types.Digest]*diff.DiffMetrics{}
	err := json.Unmarshal(byteData, &dm)
	if err != nil {
		return nil, err
	}
	return dm, nil
}
