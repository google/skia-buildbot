package absolute

import (
	"reflect"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/perf/go/stepfit"
)

func TestGetStepFitAtMid(t *testing.T) {
	unittest.SmallTest(t)
	type args struct {
		trace   []float32
		delta   float32
		percent bool
	}
	tests := []struct {
		name string
		args args
		want *stepfit.StepFit
	}{
		{
			name: "No step - absolute",
			args: args{
				trace:   []float32{1, 2, 1, 2},
				delta:   1.0,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     0,
				TurningPoint: 2,
				Regression:   0,
				Status:       stepfit.UNINTERESTING,
			},
		},
		{
			name: "No step - percent",
			args: args{
				trace:   []float32{1, 2, 1, 2},
				delta:   1.0,
				percent: true,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     0,
				TurningPoint: 2,
				Regression:   0,
				Status:       stepfit.UNINTERESTING,
			},
		},
		{
			name: "Step - absolute - exact",
			args: args{
				trace:   []float32{1, 1, 2, 2},
				delta:   1.0,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     -1.0,
				TurningPoint: 2,
				Regression:   -1.0,
				Status:       stepfit.HIGH,
			},
		},
		{
			name: "No step - absolute - too small",
			args: args{
				trace:   []float32{1, 1, 1.5, 1.5},
				delta:   1.0,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     -0.5,
				TurningPoint: 2,
				Regression:   -0.5,
				Status:       stepfit.UNINTERESTING,
			},
		},
		{
			name: "Step - absolute - big - odd",
			args: args{
				trace:   []float32{1, 1, 4, 4, 4},
				delta:   1.0,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     -3,
				TurningPoint: 2,
				Regression:   -3,
				Status:       stepfit.HIGH,
			},
		},
		{
			name: "Step - percent - big - odd",
			args: args{
				trace:   []float32{1, 1, 4, 4, 4, 4},
				delta:   0.10,
				percent: true,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     -2.0 / 3.0,
				TurningPoint: 3,
				Regression:   -2.0 / 3.0,
				Status:       stepfit.HIGH,
			},
		},
		{
			name: "Empty absolute",
			args: args{
				trace:   []float32{},
				delta:   0.10,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     0,
				TurningPoint: 0,
				Regression:   0,
				Status:       stepfit.UNINTERESTING,
			},
		},
		{
			name: "Empty percent",
			args: args{
				trace:   []float32{},
				delta:   0.10,
				percent: false,
			},
			want: &stepfit.StepFit{
				LeastSquares: 0,
				StepSize:     0,
				TurningPoint: 0,
				Regression:   0,
				Status:       stepfit.UNINTERESTING,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetStepFitAtMid(tt.args.trace, tt.args.delta, tt.args.percent); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetStepFitAtMid() = %v, want %v", got, tt.want)
			}
		})
	}
}
