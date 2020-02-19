package stepfit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/types"
)

// x values are supplied but ignored in all the tests below.
const x = vec32.MISSING_DATA_SENTINEL

// minStdDev is the minimum standard deviation used in all tests.
const minStdDev = 0.1

func TestStepFit_Empty(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 0, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_SimpleStepUp(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -2.0412414, Status: HIGH, Regression: -20.412415},
		GetStepFitAtMid([]float32{0, 0, 1, 1, 1}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_SimpleStepDown(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 2.0412414, Status: LOW, Regression: 20.412415},
		GetStepFitAtMid([]float32{1, 1, 0, 0, 0}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_NoStep(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -1, Status: UNINTERESTING, Regression: -2.7105057e-19},
		GetStepFitAtMid([]float32{1, 1, 1, 1, 1}, minStdDev, 50, types.ORIGINAL_STEP))
}

func TestStepFit_Absolute_NoStep(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{1, 2, 1, 2, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_Absolute_StepExactMatch(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -1, Status: HIGH, Regression: -1},
		GetStepFitAtMid([]float32{1, 1, 2, 2, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_Absolute_StepTooSmall(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
		GetStepFitAtMid([]float32{1, 1, 1.5, 1.5, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_Percent_NoStep(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{1, 2, 1, 2, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_Percent_StepExactMatch(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 1, StepSize: -1, Status: HIGH, Regression: -1},
		GetStepFitAtMid([]float32{1, 2, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_Percent_StepTooSmall(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 1, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
		GetStepFitAtMid([]float32{1, 1.5, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_Cohen_NoStep(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: UNINTERESTING, Regression: -0.1999998},
		GetStepFitAtMid([]float32{1, 1.1, 0.9, 1.02, 1.12, 0.92, x}, minStdDev, 1.0, types.COHEN_STEP))
}

func TestStepFit_Cohen_Step(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: HIGH, Regression: -0.1999998},
		GetStepFitAtMid([]float32{1, 1.1, 0.9, 1.02, 1.12, 0.92, x}, minStdDev, 0.1, types.COHEN_STEP))
}

func TestStepFit_Cohen_StepWithZeroStandardDeviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -10, Status: HIGH, Regression: -10},
		GetStepFitAtMid([]float32{1, 1, 2, 2, x}, minStdDev, 0.2, types.COHEN_STEP))
}
func TestStepFit_Cohen_StepWithLargeStandardDeviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{LeastSquares: 0, TurningPoint: 2, StepSize: -2.828427, Regression: -2.828427, Status: "High"},
		GetStepFitAtMid([]float32{1, 2, 3, 4, x}, minStdDev, 0.2, types.COHEN_STEP))
}
