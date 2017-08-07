package diffstore

import (
	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
)

const (
	// METRICSDB_NAME is the name of the boltdb caching diff metrics.
	METRICSDB_NAME = "diffstore_metrics"

	// METRICS_DIGEST_INDEX is the index name to keep track of digests in the metrics db.
	METRICS_DIGEST_INDEX = "metric_digest_index"
)

var (
	// metricsRecIndices are the indices supported by the metricsRec type.
	metricsRecIndices = []string{METRICS_DIGEST_INDEX}

	// metricsRecSplitFn is called to get the two image IDs from a metricsRec ID.
	metricsRecSplitFn func(string) (string, string) = nil
)

// metricsStore stores diff metrics on disk.
type metricsStore struct {
	// store stores the diff metrics in a boltdb database.
	store *boltutil.IndexedBucket

	// codec is used to encode/decode the DiffMetrics field of a metricsRec struct
	codec util.LRUCodec
}

// metricsRec implements the boltutil.Record interface.
type metricsRec struct {
	ID          string `json:"id"`
	DiffMetrics []byte
}

// Key see the boltutil.Record interface.
func (m *metricsRec) Key() string {
	return m.ID
}

// IndexValues see the boltutil.Record interface.
func (m *metricsRec) IndexValues() map[string][]string {
	d1, d2 := metricsRecSplitFn(m.ID)
	return map[string][]string{METRICS_DIGEST_INDEX: {d1, d2}}
}

// newMetricsStore returns a new instance of metricsStore.
func newMetricsStore(baseDir string, splitFn func(string) (string, string), codec util.LRUCodec) (*metricsStore, error) {
	metricsRecSplitFn = splitFn

	db, err := openBoltDB(baseDir, METRICSDB_NAME+".db")
	if err != nil {
		return nil, err
	}

	config := &boltutil.Config{
		DB:      db,
		Name:    METRICSDB_NAME,
		Indices: metricsRecIndices,
		Codec:   util.JSONCodec(&metricsRec{}),
	}

	store, err := boltutil.NewIndexedBucket(config)
	if err != nil {
		return nil, err
	}

	return &metricsStore{
		store: store,
		codec: codec,
	}, nil
}

// loadDiffMetrics loads diff metrics from disk.
func (m *metricsStore) loadDiffMetrics(id string) (interface{}, error) {
	recs, err := m.store.Read([]string{id})
	if err != nil {
		return nil, err
	}

	if recs[0] == nil {
		return nil, nil
	}

	// Deserialize the byte array representing the DiffMetrics.
	diffMetrics, err := m.codec.Decode(recs[0].(*metricsRec).DiffMetrics)
	if err != nil {
		return nil, err
	}
	return diffMetrics, nil
}

// saveDiffMetrics stores diff metrics to disk.
func (m *metricsStore) saveDiffMetrics(id string, diffMetrics interface{}) error {
	// Serialize the diffMetrics.
	bytes, err := m.codec.Encode(diffMetrics)
	if err != nil {
		return err
	}

	rec := &metricsRec{ID: id, DiffMetrics: bytes}
	return m.store.Insert([]boltutil.Record{rec})
}

// purgeDiffMetrics removes all diff metrics based on specific digests.
func (m *metricsStore) purgeDiffMetrics(digests []string) error {
	updateFn := func(tx *bolt.Tx) error {
		metricIDMap, err := m.store.ReadIndexTx(tx, METRICS_DIGEST_INDEX, digests)
		if err != nil {
			return err
		}

		metricIds := util.StringSet{}
		for _, ids := range metricIDMap {
			metricIds.AddLists(ids)
		}

		return m.store.DeleteTx(tx, metricIds.Keys())
	}

	return m.store.DB.Update(updateFn)
}
