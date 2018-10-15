package regression

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
)

func TestRegressions(t *testing.T) {
	testutils.SmallTest(t)
	r := New()
	assert.True(t, r.Triaged(), "With no clusters, it should have Triaged() == true.")

	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("source_type=skp", df, cl)
	assert.False(t, r.Triaged(), "Should not be Triaged.")

	// Triage the low cluster.
	err := r.TriageLow("source_type=skp", TriageStatus{
		Status:  POSITIVE,
		Message: "SKP Update",
	})
	assert.NoError(t, err)
	assert.True(t, r.Triaged())

	// Trying to triage a high cluster that doesn't exists.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status: NEGATIVE,
	})
	assert.Equal(t, err, ErrNoClusterFound)
	assert.True(t, r.Triaged())

	// Set a high cluster.
	r.SetHigh("source_type=skp", df, cl)
	assert.False(t, r.Triaged())

	// And triage the high cluster.
	err = r.TriageHigh("source_type=skp", TriageStatus{
		Status:  NEGATIVE,
		Message: "See bug #foo.",
	})
	assert.NoError(t, err)
	assert.True(t, r.Triaged())

	// Trying to triage an unknown query.
	err = r.TriageHigh("uknownquery", TriageStatus{
		Status: NEGATIVE,
	})
	assert.Equal(t, err, ErrNoClusterFound)

	// Try serializing to JSON.
	b, err := r.JSON()
	assert.NoError(t, err)
	assert.Equal(t, "{\"by_query\":{\"source_type=skp\":{\"low\":{\"centroid\":null,\"keys\":null,\"param_summaries\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"high\":{\"centroid\":null,\"keys\":null,\"param_summaries\":null,\"step_fit\":null,\"step_point\":null,\"num\":0},\"frame\":{\"dataframe\":null,\"ticks\":null,\"skps\":null,\"msg\":\"\"},\"low_status\":{\"status\":\"positive\",\"message\":\"SKP Update\"},\"high_status\":{\"status\":\"negative\",\"message\":\"See bug #foo.\"}}}}", string(b))
}
