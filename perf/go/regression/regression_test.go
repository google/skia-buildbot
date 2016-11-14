package regression

import (
	"testing"

	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"

	"github.com/stretchr/testify/assert"
)

func TestRegressions(t *testing.T) {
	r := New()
	assert.False(t, r.Untriaged(), "With no clusters, it should has Untriaged() == false.")

	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("source_type=skp", df, cl)
	assert.True(t, r.Untriaged(), "Should now be Untriaged.")

	// Triage the low cluster.
	err := r.TriageLow("source_type=skp", TriageStatus{
		Status:  IGNORE,
		Message: "SKP Update",
	})
	assert.NoError(t, err)
	assert.False(t, r.Untriaged())

	// Trying to triage a high cluster that doesn't exists.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status: BUG,
	})
	assert.Equal(t, err, ErrNoClusterFound)

	// Set a high cluster.
	r.SetHigh("source_type=skp", df, cl)
	assert.True(t, r.Untriaged())

	// And triage the high cluster.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status:  BUG,
		Message: "See bug #foo.",
	})
	assert.NoError(t, err)
	assert.False(t, r.Untriaged())

	// Trying to triage an unknown query.
	err = r.TriageHigh("uknownquery", TriageStatus{
		Status: BUG,
	})
	assert.Equal(t, err, ErrNoClusterFound)

	// Try serializing to JSON.
	b, err := r.JSON()
	assert.NoError(t, err)
	assert.Equal(t, "{\"by_query\":{\"source_type=skp\":{\"low\":{\"centroid\":null,\"keys\":null,\"param_summaries\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"high\":{\"centroid\":null,\"keys\":null,\"param_summaries\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"frame\":{\"dataframe\":null,\"ticks\":null,\"skps\":null,\"msg\":\"\"},\"low_status\":{\"status\":\"Ignore\",\"message\":\"SKP Update\"},\"high_status\":{\"status\":\"Bug\",\"message\":\"See bug #foo.\"}}}}", string(b))
}
