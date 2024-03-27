package internal

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.temporal.io/sdk/testsuite"
)

func TestComparePerformance_AsActivity_ShouldEqualComparePerformance(t *testing.T) {
	test := func(name string, valuesA, valuesB []float64, magnitude float64) {
		t.Run(name, func(t *testing.T) {
			expected, err := compare.ComparePerformance(valuesA, valuesB, magnitude)
			require.NoError(t, err)

			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()
			env.RegisterActivity(ComparePerformanceActivity)
			res, err := env.ExecuteActivity(ComparePerformanceActivity, valuesA, valuesB, magnitude)
			require.NoError(t, err)

			var actual *compare.CompareResults
			err = res.Get(&actual)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
	// based off of Pinpoint job: https://pinpoint-dot-chromeperf.appspot.com/job/1646b907ae0000
	// can see the results here: https://pinpoint-dot-chromeperf.appspot.com/api/job/1646b907ae0000?o=STATE&o=ESTIMATE
	valuesA := []float64{35.54, 34.799, 32.397, 35.373, 37.256, 32.199, 41.761, 33.616, 34.863, 34.588}
	valuesB := []float64{36.176, 38.124, 34.07, 35.3, 33.921, 34.472, 33.264, 36.727, 38.353, 33.941}
	magnitude := 11.8
	test("CompareResults verdict same", valuesA, valuesB, magnitude)

	valuesA = []float64{35.54, 34.799, 32.397, 35.373, 37.256, 32.199, 41.761, 33.616, 34.863, 34.588}
	valuesB = []float64{42.226, 45.616, 37.242, 48.362, 42.206, 44.049, 42.933, 51.292, 50.884, 40.601}
	test("CompareResults verdict different", valuesA, valuesB, magnitude)

	// based off of Pinpoint job: https://pinpoint-dot-chromeperf.appspot.com/job/138f08c29e0000
	// can see results here: https://pinpoint-dot-chromeperf.appspot.com/api/job/138f08c29e0000?o=STATE&o=ESTIMATE
	valuesA = []float64{1022.2335803760349, 1021.8418699774292, 1022.4948875275097, 1029.9987125001273, 1013.6847440431667, 1045.0685826282777, 1041.8023180121793, 1041.2599245076458, 1033.725287504348, 1031.9917440670392, 1035.8668911084903}
	valuesB = []float64{1041.5310506525295, 1040.1768300490185, 1044.3864229714222, 1030.2640051562628, 1026.9576379925215, 1071.4720565094074, 1071.0808179337444, 1065.388200848932, 1064.1127959605897, 1061.993893518306, 1066.524463397464}
	magnitude = 1.0
	test("CompareResults verdict unknown", valuesA, valuesB, magnitude)

}

func TestCompareFunctional_AsActivity_ShouldEqualCompareFunctional(t *testing.T) {
	test := func(name string, valuesA, valuesB []float64, errRate float64) {
		t.Run(name, func(t *testing.T) {
			expected, err := compare.CompareFunctional(valuesA, valuesB, errRate)
			require.NoError(t, err)

			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestActivityEnvironment()
			env.RegisterActivity(CompareFunctionalActivity)
			res, err := env.ExecuteActivity(CompareFunctionalActivity, valuesA, valuesB, errRate)
			require.NoError(t, err)

			var actual *compare.CompareResults
			err = res.Get(&actual)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
		})
	}
	x := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	y := []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	test("arrays are slightly different, return unknown", x, y, 0.5)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	test("arrays are the same, return same", x, y, 1.0)

	x = []float64{0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	y = []float64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	test("arrays are significantly different, return different", x, y, 1.0)
}
