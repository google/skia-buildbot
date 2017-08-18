package diffstore

import (
	"encoding/json"
	"fmt"

	"github.com/boltdb/bolt"
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
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

	// TODO(stephana): Remove the database guard below when we don't need it anymore.
	// get the record and check if it's a legacy entry.
	rec := recs[0].(*metricsRec)
	if len(rec.DiffMetrics) == 0 {
		if diffMetrics := m.fixLegacyRecord(id); diffMetrics != nil {
			return diffMetrics, nil
		}
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

	if len(bytes) == 0 {
		return fmt.Errorf("Got empty string for encoded diff metric.")
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

// legacyMetricsRec is the old format of the metrics records. Used by the
// db guard below.
type legacyMetricsRec struct {
	ID string `json:"id"`
	*diff.DiffMetrics
}

func (m *metricsStore) fixLegacyRecord(id string) *diff.DiffMetrics {
	// Read the bytes from the db.
	contentBytes, err := m.store.ReadRaw(id)
	if err != nil {
		sklog.Errorf("Error reading raw legacy record: %s", err)
		return nil
	}

	legRec := legacyMetricsRec{}
	if err := json.Unmarshal(contentBytes, &legRec); err != nil {
		sklog.Errorf("Unable to decode legacy error: %s", err)
		return nil
	}

	// Use simple heuristic to figure whether we have a record.
	if len(legRec.DiffMetrics.MaxRGBADiffs) != 4 {
		sklog.Errorf("Did not get a valid legacy diff metrics record.")
		return nil
	}

	// Write the new record to the database in the background.
	go func() {
		if err := m.saveDiffMetrics(id, legRec.DiffMetrics); err != nil {
			sklog.Errorf("Error writing legacy record to DB: %s", err)
		}
		sklog.Infof("Legacy database record (%s) written to the database.", id)
	}()

	return legRec.DiffMetrics
}

// convertDatabaseFromLegacy iterates over the entire database in the background
// and loads every entry, implicitly forcing a conversion to the new serialization format.
func (m *metricsStore) convertDatabaseFromLegacy() {
	go func() {
		ids, err := m.listIDs()
		if err != nil {
			sklog.Errorf("Unable to get the database ids. Got error: %s", err)
		}

		for _, id := range ids {
			// The call to loadDiffMetrics will also convert a legacy record to the new record if necessary.
			if _, err := m.loadDiffMetrics(id); err != nil {
				sklog.Errorf("Error trying to convert legacy record: %s", err)
			}
		}
		sklog.Infof("Legacy conversion: Loaded %d records.", len(ids))
		if err := m.store.ReIndex(); err != nil {
			sklog.Errorf("Error re-indexing data store: %s", err)
		}
		sklog.Infof("Legacy conversion completed.")
	}()
}

// listIDs returns a slice with all the ids in the database.
func (m *metricsStore) listIDs() ([]string, error) {
	ret := []string{}
	viewFn := func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(METRICSDB_NAME))
		if b == nil {
			return nil
		}

		ret = make([]string, 0, b.Stats().KeyN)
		c := b.Cursor()

		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			ret = append(ret, string(append([]byte(nil), k...)))
		}
		return nil
	}

	if err := m.store.DB.View(viewFn); err != nil {
		return nil, err
	}
	return ret, nil
}
