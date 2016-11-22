package diffstore

import (
	"go.skia.org/infra/go/boltutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

const (
	METRICS_DB           = "diffstore_metrics"
	METRICS_DIGEST_INDEX = "metric_digest_index"
)

type metricsStore struct {
	// metricDB stores the diff metrics in a boltdb databasel.
	store *boltutil.Store
}

// metricsRec implements the boltutil.Record interface.
type metricsRec struct {
	ID string `json:"id"`
	*diff.DiffMetrics
}

func (m *metricsRec) Key() string {
	return m.ID
}

func (m *metricsRec) IndexValues(indices []string) map[string][]string {
	ret := make(map[string][]string, len(indices))
	for _, idx := range indices {
		switch idx {
		case METRICS_DIGEST_INDEX:
			d1, d2 := splitDigests(m.ID)
			ret[idx] = append(ret[idx], d1, d2)
		}
	}
	return ret
}

func newMetricStore(baseDir string) (*metricsStore, error) {
	config := &boltutil.Config{
		Directory: baseDir,
		Name:      "diffstore_metrics",
		Indices:   []string{METRICS_DIGEST_INDEX},
		Codec:     util.JSONCodec(&metricsRec{}),
	}
	store, err := boltutil.NewStore(config)
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

// saveDiffMetric stores a diffmetric on disk.
func (m *metricsStore) saveDiffMetric(id string, dr *diff.DiffMetrics) error {
	rec := &metricsRec{ID: id, DiffMetrics: dr}
	return m.store.Write([]boltutil.Record{rec}, nil)
}

func (m *metricsStore) purgeMetrics(digests []string) error {
	return nil
}
