package diffstore

import (
	"fmt"

	"github.com/boltdb/bolt"

	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

type metricsStore struct {
	// metricDB stores the diff metrics in a boltdb databasel.
	metricsDB *bolt.DB

	// diffMetricsCodec encodes/decodes diff.DiffMetrics instances to JSON.
	diffMetricsCodec util.LRUCodec
}

const INDEX_DIGEST = "idx-digests"

type metricsRec struct {
	*diff.DiffMetrics
}

func (*metricsRec) IndexVals(idx string) []string {
	return nil
}

func newMetricStore(directory, name string) (*metricsStore, error) {
	config := &boltutil.Config{
		Directory: directory,
		Name:      name,
		Indices:   []string{INDEX_DIGEST},
	}

	metricsDB, err := boltutil.Open(fileName, 0600, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to open metricsDB at %s. Got error: %s", fileName, err)
	}

	return &metricsStore{
		metricsDB:        metricsDB,
		diffMetricsCodec: util.JSONCodec(&diff.DiffMetrics{}),
	}, nil
}

// loadDiffMetric loads a diffMetric from disk.
func (m *metricsStore) loadDiffMetric(id string) (*diff.DiffMetrics, error) {
	var jsonData []byte = nil
	viewFn := func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(METRICS_BUCKET))
		if bucket == nil {
			return nil
		}

		jsonData = bucket.Get([]byte(id))
		return nil
	}

	if err := m.metricsDB.View(viewFn); err != nil {
		return nil, err
	}

	if jsonData == nil {
		return nil, nil
	}

	ret, err := m.diffMetricsCodec.Decode(jsonData)
	if err != nil {
		return nil, err
	}
	return ret.(*diff.DiffMetrics), nil
}

// saveDiffMetric stores a diffmetric on disk.
func (m *metricsStore) saveDiffMetric(id string, dr *diff.DiffMetrics) error {
	jsonData, err := m.diffMetricsCodec.Encode(dr)
	if err != nil {
		return err
	}

	updateFn := func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(METRICS_BUCKET))
		if err != nil {
			return err
		}

		return bucket.Put([]byte(id), jsonData)
	}

	err = m.metricsDB.Update(updateFn)
	return err
}

func (m *metricsStore) purgeMetrics(digests []string) error {

	return nil
}
