package regression_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/dfiter"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regression/refiner"
	"go.skia.org/infra/perf/go/types"
)

func TestIntegration_between_DetectRegressionsOnDataFrame_and_AnomalyBoundsRefiner(t *testing.T) {
	ctx := context.Background()

	// 1. Create a mocked DataFrame with the specific trace values
	jsonData := testutils.ReadFileBytes(t, "refiner_integration_trace.json")

	var parsedData struct {
		TraceSet map[string][]float32 `json:"traceset"`
	}
	require.NoError(t, json.Unmarshal(jsonData, &parsedData))

	traceSet := types.TraceSet{}
	maxLen := 0
	for k, v := range parsedData.TraceSet {
		traceSet[k] = types.Trace(v)
		if len(v) > maxLen {
			maxLen = len(v)
		}
	}

	df := &dataframe.DataFrame{
		Header:   []*dataframe.ColumnHeader{},
		TraceSet: traceSet,
		ParamSet: paramtools.ReadOnlyParamSet{},
	}

	// Create column headers to match the trace length
	for i := 0; i < maxLen; i++ {
		df.Header = append(df.Header, &dataframe.ColumnHeader{
			Offset:    types.CommitNumber(i),
			Timestamp: dataframe.TimestampSeconds(time.Now().Add(time.Duration(i) * time.Hour).Unix()),
		})
	}

	// 2. Setup the Alert Configuration
	alertConfig := &alerts.Alert{
		Radius:            8,                     // Using a small radius since trace is short
		Interesting:       2.5,                   // Threshold for the regression detection
		DirectionAsString: alerts.BOTH,           // Assuming we're looking for an upward regression (10 -> 16)
		Algo:              types.StepFitGrouping, // types.RegressionDetectionGrouping
		Step:              types.CohenStep,       // types.StepDetection
	}

	// 3. Setup the detector process
	// We need to initialize config to avoid panic in progress.New()
	// and to allow dfiter to slice appropriately.
	config.Config = &config.InstanceConfig{
		Experiments: config.Experiments{
			DfIterTraceSlicer: true,
		},
	}

	prog := progress.New()
	iter := dfiter.NewStepFitDfTraceSlicer(df, alertConfig.Radius)

	// Use the real refiner for the integration test
	realRefiner := refiner.NewAnomalyBoundsRefiner(0.1)

	p := regression.NewExportedRegressionDetectionProcess(prog, iter, alertConfig, realRefiner)

	// 4. Run Step 1: detectRegressionsOnDataFrame using the iterator
	var allResponses []*regression.RegressionDetectionResponse

	for iter.Next() {
		slicedDf, err := iter.Value(ctx)
		require.NoError(t, err)

		resp, err := p.DetectRegressionsOnDataFrame(ctx, slicedDf)
		require.NoError(t, err)

		if resp != nil {
			allResponses = append(allResponses, resp)
		}
	}

	// 5. Run Step 2: Pass output to RegressionRefiner
	confirmed, err := realRefiner.Process(ctx, alertConfig, allResponses)

	// 6. Verify final output
	require.NoError(t, err)

	prevCommitNumbers := []int{15, 21, 43, 154, 202, 212, 222, 241}
	curtCommitNumbers := []int{16, 22, 44, 155, 203, 213, 223, 242}

	// We should see a consolidated number of anomalies
	assert.Equal(t, len(curtCommitNumbers), len(confirmed))
	// Sanity check against malformed tests
	assert.Equal(t, len(curtCommitNumbers), len(prevCommitNumbers))

	for i, cn := range curtCommitNumbers {
		assert.Equal(t, types.CommitNumber(prevCommitNumbers[i]), confirmed[i].PrevCommitNumber)
		assert.Equal(t, types.CommitNumber(cn), confirmed[i].CommitNumber)
	}
}
