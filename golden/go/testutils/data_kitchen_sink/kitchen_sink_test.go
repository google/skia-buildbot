package data_kitchen_sink_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/golden/go/tiling"

	. "go.skia.org/infra/golden/go/testutils/data_kitchen_sink"
)

func TestMakeTraces_CorrectIDs(t *testing.T) {
	traces := MakeTraces()
	uniqueIds := map[tiling.TraceID]bool{}
	for _, tp := range traces {
		assert.Equal(t, tiling.TraceIDFromParams(tp.Trace.Keys()), tp.ID)
		assert.NotContains(t, uniqueIds, tp.ID, "traces should be unique - %s was not", tp.ID)
		uniqueIds[tp.ID] = true
	}
}

func TestMakeTraces_CorrectNumberOfDigests(t *testing.T) {
	traces := MakeTraces()
	for _, tp := range traces {
		assert.Len(t, tp.Trace.Digests, NumCommits)
		assert.Equal(t, NumCommits, tp.Trace.Len())
	}
}
