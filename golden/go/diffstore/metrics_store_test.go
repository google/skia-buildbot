package diffstore

import (
	"fmt"
	"testing"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"

	assert "github.com/stretchr/testify/require"
)

func TestMetricsStore(t *testing.T) {
	mapper := GoldIDPathMapper{}
	codec := util.JSONCodec(&diff.DiffMetrics{})
	store, err := newMetricsStore(".", mapper.SplitDiffID, codec)
	assert.NoError(t, err)

	ids, err := store.listIDs()
	assert.NoError(t, err)
	assert.NotEmpty(t, ids)

	for _, id := range ids {
		dm, err := store.loadDiffMetrics(id)

		if err != nil {
			fmt.Printf("Errorf: %s\n", err)
			rawRecs, err := store.store.Read([]string{id})
			assert.NoError(t, err)

			jsonStr := string(rawRecs[0].(*metricsRec).DiffMetrics)
			fmt.Printf("CONTENT for %s (%d): \n%s\n", id, len(jsonStr), jsonStr)
		}

		assert.NoError(t, err)
		_, ok := dm.(*diff.DiffMetrics)
		assert.True(t, ok)
	}
}
