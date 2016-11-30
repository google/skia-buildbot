package diffstore

import (
	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	// METRICSDB_NAME is the name of the boltdb caching diff metrics.
	METRICSDB_NAME = "diffstore_metrics"

	// METRICS_DIGEST_INDEX is the index name to keep track of digests in the metrics db.
	METRICS_DIGEST_INDEX = "metric_digest_index"
)

// metricsStore stores diff metrics on disk.
type metricsStore struct {
	// store stores the diff metrics in a boltdb database.
	store *boltutil.IndexedBucket
}

// metricsRecIndices are  the indices supported by the metricsRec type.
var metricsRecIndices = []string{METRICS_DIGEST_INDEX}

// metricsRec implements the boltutil.Record interface.
type metricsRec struct {
	ID string `json:"id"`
	*diff.DiffMetrics
}

// Key see the boltutil.Record interface.
func (m *metricsRec) Key() string {
	return m.ID
}

// IndexValues see the boltutil.Record interface.
func (m *metricsRec) IndexValues() map[string][]string {
	d1, d2 := splitDigests(m.ID)
	return map[string][]string{METRICS_DIGEST_INDEX: {d1, d2}}
}

// newMetricsStore returns a new instance of metricsStore.
func newMetricStore(baseDir string) (*metricsStore, error) {
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
	}, nil
}

// loadDiffMetric loads a diffMetric from disk.
func (m *metricsStore) loadDiffMetric(id string) (*diff.DiffMetrics, error) {
	recs, err := m.store.Read([]string{id})
	if err != nil {
		return nil, err
	}

	if recs[0] == nil {
		return nil, nil
	}
	return recs[0].(*metricsRec).DiffMetrics, nil
}

// saveDiffMetric stores a diffmetric to disk.
func (m *metricsStore) saveDiffMetric(id string, dr *diff.DiffMetrics) error {
	rec := &metricsRec{ID: id, DiffMetrics: dr}
	return m.store.Insert([]boltutil.Record{rec})
}

// purgeMetrics removes all metrics that based on a specific digest.
func (m *metricsStore) purgeMetrics(digests []string) error {
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
