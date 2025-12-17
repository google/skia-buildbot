package compat

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/chromeperf"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/ui/frame"
)

func TestConvertRegressionToAnomalies_Success(t *testing.T) {
	reg := &regression.Regression{
		Id:               "test_regression",
		CommitNumber:     12345,
		PrevCommitNumber: 12340,
		IsImprovement:    false,
		MedianBefore:     10.0,
		MedianAfter:      20.0,
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				TraceSet: map[string]types.Trace{
					",master=ChromiumPerf,bot=mac-m1,benchmark=MyBench,test=MyTest,subtest_1=sub1,": {},
				},
			},
		},
	}

	anomalies, err := ConvertRegressionToAnomalies(reg)
	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	assert.Len(t, anomalies, 1)

	key := ",master=ChromiumPerf,bot=mac-m1,benchmark=MyBench,test=MyTest,subtest_1=sub1,"
	assert.Contains(t, anomalies, key)

	commitMap := anomalies[key]
	assert.Len(t, commitMap, 1)
	assert.Contains(t, commitMap, types.CommitNumber(12345))

	anomaly := commitMap[types.CommitNumber(12345)]
	assert.Equal(t, "test_regression", anomaly.Id)
	assert.Equal(t, "ChromiumPerf/mac-m1/MyBench/MyTest/sub1", anomaly.TestPath)
	assert.Equal(t, 12340, anomaly.StartRevision)
	assert.Equal(t, 12345, anomaly.EndRevision)
	assert.False(t, anomaly.IsImprovement)
	assert.Equal(t, 10.0, anomaly.MedianBeforeAnomaly)
	assert.Equal(t, 20.0, anomaly.MedianAfterAnomaly)
}

func TestConvertRegressionToAnomalies_NilFrame(t *testing.T) {
	reg := &regression.Regression{
		Id: "test_regression",
	}

	anomalies, err := ConvertRegressionToAnomalies(reg)
	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	assert.Empty(t, anomalies)
}

func TestConvertRegressionToAnomalies_NilDataFrame(t *testing.T) {
	reg := &regression.Regression{
		Id:    "test_regression",
		Frame: &frame.FrameResponse{},
	}

	anomalies, err := ConvertRegressionToAnomalies(reg)
	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	assert.Empty(t, anomalies)
}

func TestConvertRegressionToAnomalies_TraceNameError(t *testing.T) {
	reg := &regression.Regression{
		Id: "test_regression",
		Frame: &frame.FrameResponse{
			DataFrame: &dataframe.DataFrame{
				TraceSet: map[string]types.Trace{
					"invalid-trace-name": {},
				},
			},
		},
	}

	anomalies, err := ConvertRegressionToAnomalies(reg)
	assert.NoError(t, err)
	assert.NotNil(t, anomalies)
	assert.Empty(t, anomalies)
}

func TestConvertRegressionToAnomalies_Status(t *testing.T) {
	tests := []struct {
		name          string
		triageStatus  regression.Status
		expectedBugId int
	}{
		{
			name:          "Ignored",
			triageStatus:  regression.Ignored,
			expectedBugId: chromeperf.IgnoreBugIDFlag,
		},
		{
			name:          "Untriaged",
			triageStatus:  regression.Untriaged,
			expectedBugId: 0,
		},
		{
			name:          "Positive",
			triageStatus:  regression.Positive,
			expectedBugId: 0,
		},
		{
			name:          "None",
			triageStatus:  regression.None,
			expectedBugId: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &regression.Regression{
				Id:               "test_regression_" + tt.name,
				CommitNumber:     12345,
				PrevCommitNumber: 12340,
				IsImprovement:    false,
				MedianBefore:     10.0,
				MedianAfter:      20.0,
				Frame: &frame.FrameResponse{
					DataFrame: &dataframe.DataFrame{
						TraceSet: map[string]types.Trace{
							",master=ChromiumPerf,bot=mac-m1,benchmark=MyBench,test=MyTest,subtest_1=sub1,": {},
						},
					},
				},
				Low: &clustering2.ClusterSummary{},
				LowStatus: regression.TriageStatus{
					Status: tt.triageStatus,
				},
			}

			anomalies, err := ConvertRegressionToAnomalies(reg)
			assert.NoError(t, err)
			assert.NotNil(t, anomalies)

			key := ",master=ChromiumPerf,bot=mac-m1,benchmark=MyBench,test=MyTest,subtest_1=sub1,"
			commitMap := anomalies[key]
			anomaly := commitMap[types.CommitNumber(12345)]

			assert.Equal(t, string(tt.triageStatus), anomaly.State)
			assert.Equal(t, tt.expectedBugId, anomaly.BugId)
		})
	}
}
