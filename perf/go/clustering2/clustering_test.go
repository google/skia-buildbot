package clustering2

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/ctrace2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/kmeans"
	"go.skia.org/infra/perf/go/ptracestore"
)

func TestParamSummaries(t *testing.T) {
	obs := []kmeans.Clusterable{
		ctrace2.NewFullTrace(",arch=x86,config=8888,", []float32{1, 2}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{2, 3}, 0.001),
		ctrace2.NewFullTrace(",arch=x86,config=565,", []float32{3, 2}, 0.001),
	}
	expected := map[string][]ValueWeight{
		"arch": []ValueWeight{
			{"x86", 26},
		},
		"config": []ValueWeight{
			{"565", 21},
			{"8888", 16},
		},
	}
	assert.Equal(t, expected, getParamSummaries(obs))

	obs = []kmeans.Clusterable{}
	expected = map[string][]ValueWeight{}
	assert.Equal(t, expected, getParamSummaries(obs))
}

func TestStepFit(t *testing.T) {
	testCases := []struct {
		value    []float32
		expected *StepFit
		message  string
	}{
		{
			value:    []float32{0, 0, 1, 1, 1},
			expected: &StepFit{TurningPoint: 2, StepSize: -1, Status: "Low"},
			message:  "Simple Step Up",
		},
		{
			value:    []float32{1, 1, 1, 0, 0},
			expected: &StepFit{TurningPoint: 3, StepSize: 1, Status: "High"},
			message:  "Simple Step Down",
		},
		{
			value:    []float32{1, 1, 1, 1, 1},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: "Uninteresting"},
			message:  "No step",
		},
		{
			value:    []float32{},
			expected: &StepFit{TurningPoint: 0, StepSize: -1, Status: "Uninteresting"},
			message:  "Empty",
		},
	}

	for _, tc := range testCases {
		got, want := getStepFit(tc.value), tc.expected
		if got.StepSize != want.StepSize {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.StepSize, want.StepSize, tc.message)
		}
		if got.Status != want.Status {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.Status, want.Status, tc.message)
		}
		if got.TurningPoint != want.TurningPoint {
			t.Errorf("Failed StepFit Got %#v Want %#v: %s", got.TurningPoint, want.TurningPoint, tc.message)
		}
	}
}

func TestCalcCusterSummaries(t *testing.T) {
	rand.Seed(1)
	now := time.Now()
	df := &dataframe.DataFrame{
		TraceSet: ptracestore.TraceSet{
			",arch=x86,config=8888,": []float32{0, 0, 0, 1, 1},
			",arch=x86,config=565,":  []float32{0, 0, 0, 1, 1},
			",arch=arm,config=8888,": []float32{1, 1, 1, 1, 1},
			",arch=arm,config=565,":  []float32{1, 1, 1, 1, 1},
		},
		Header: []*dataframe.ColumnHeader{
			{
				Source:    "master",
				Offset:    0,
				Timestamp: now.Unix(),
			},
			{
				Source:    "master",
				Offset:    1,
				Timestamp: now.Add(time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    2,
				Timestamp: now.Add(2 * time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    3,
				Timestamp: now.Add(3 * time.Minute).Unix(),
			},
			{
				Source:    "master",
				Offset:    4,
				Timestamp: now.Add(4 * time.Minute).Unix(),
			},
		},
		ParamSet: paramtools.ParamSet{},
		Skip:     0,
	}
	for key, _ := range df.TraceSet {
		df.ParamSet.AddParamsFromKey(key)
	}
	sum, err := CalculateClusterSummaries(df, 4, 0.01, nil)
	assert.NoError(t, err)
	assert.NotNil(t, sum)
	assert.Equal(t, 2, len(sum.Clusters))
	assert.Equal(t, df.Header[3], sum.Clusters[0].StepPoint)
	assert.Equal(t, 2, len(sum.Clusters[0].Keys))
	assert.Equal(t, 2, len(sum.Clusters[1].Keys))
}

func TestCalcCusterSummariesDegenerate(t *testing.T) {
	rand.Seed(1)
	df := &dataframe.DataFrame{
		TraceSet: ptracestore.TraceSet{},
		Header:   []*dataframe.ColumnHeader{},
		ParamSet: paramtools.ParamSet{},
		Skip:     0,
	}
	_, err := CalculateClusterSummaries(df, 4, 0.01, nil)
	assert.Error(t, err)
}
