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

func TestStepFit_empty(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 0, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_simple_step_up(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -2.0412414, Status: HIGH, Regression: -20.412415},
		GetStepFitAtMid([]float32{0, 0, 1, 1, 1}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_simple_step_down(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 2.0412414, Status: LOW, Regression: 20.412415},
		GetStepFitAtMid([]float32{1, 1, 0, 0, 0}, minStdDev, 20, types.ORIGINAL_STEP))
}

func TestStepFit_no_step(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -1, Status: UNINTERESTING, Regression: -2.7105057e-19},
		GetStepFitAtMid([]float32{1, 1, 1, 1, 1}, minStdDev, 50, types.ORIGINAL_STEP))
}

func TestStepFit_absolute_no_step(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{1, 2, 1, 2, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_absolute_step_exact_match(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -1, Status: HIGH, Regression: -1},
		GetStepFitAtMid([]float32{1, 1, 2, 2, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_absolute_no_step_too_small(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
		GetStepFitAtMid([]float32{1, 1, 1.5, 1.5, x}, minStdDev, 1.0, types.ABSOLUTE_STEP))
}

func TestStepFit_percent_no_step(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: 0, Status: UNINTERESTING, Regression: 0},
		GetStepFitAtMid([]float32{1, 2, 1, 2, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_percent_step_exact(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 1, StepSize: -1, Status: HIGH, Regression: -1},
		GetStepFitAtMid([]float32{1, 2, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_percent_no_step_too_small(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 1, StepSize: -0.5, Status: UNINTERESTING, Regression: -0.5},
		GetStepFitAtMid([]float32{1, 1.5, x}, minStdDev, 1.0, types.PERCENT_STEP))
}

func TestStepFit_cohen_no_step_small_standard_deviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: UNINTERESTING, Regression: -0.1999998},
		GetStepFitAtMid([]float32{1, 1.1, 0.9, 1.02, 1.12, 0.92, x}, minStdDev, 1.0, types.COHEN_STEP))
}

func TestStepFit_cohen_step_small_standard_deviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 3, StepSize: -0.1999998, Status: HIGH, Regression: -0.1999998},
		GetStepFitAtMid([]float32{1, 1.1, 0.9, 1.02, 1.12, 0.92, x}, minStdDev, 0.1, types.COHEN_STEP))
}

func TestStepFit_cohen_step_zero_standard_deviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{TurningPoint: 2, StepSize: -10, Status: HIGH, Regression: -10},
		GetStepFitAtMid([]float32{1, 1, 2, 2, x}, minStdDev, 0.2, types.COHEN_STEP))
}
func TestStepFit_cohen_step_large_standard_deviation(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t,
		&StepFit{LeastSquares: 0, TurningPoint: 2, StepSize: -2.828427, Regression: -2.828427, Status: "High"},
		GetStepFitAtMid([]float32{1, 2, 3, 4, x}, minStdDev, 0.2, types.COHEN_STEP))
}
