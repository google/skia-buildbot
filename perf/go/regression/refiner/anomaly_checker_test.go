package refiner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestIsAnomaly(t *testing.T) {
	stdDevThreshold := float32(0.001)

	tests := []struct {
		name     string
		val      float32
		baseline []float32
		algo     types.StepDetection
		interest float32
		want     bool
	}{
		{
			name:     "Not enough baseline data (cohen)",
			val:      10.0,
			baseline: []float32{100.0},
			algo:     types.CohenStep,
			interest: 2.0,
			want:     false, // Unexpected scenario (insufficient baseline data). CohenStep safely handles inputs with <4 points by returning 0, which correctly evaluates as not an anomaly.
		},
		{
			name:     "Not enough baseline data (Const Step)",
			val:      10.0,
			baseline: []float32{100.0},
			algo:     types.Const,
			interest: 9.0,
			want:     true, // Const step doesn't use baseline data
		},
		{
			name:     "Absolute Step - Anomaly",
			val:      20.0,
			baseline: []float32{10.0, 10.0, 10.0},
			algo:     types.AbsoluteStep,
			interest: 5.0,
			want:     true, // |20 - 10| = 10 >= 5
		},
		{
			name:     "Absolute Step - Normal",
			val:      12.0,
			baseline: []float32{10.0, 10.0, 10.0},
			algo:     types.AbsoluteStep,
			interest: 5.0,
			want:     false, // |12 - 10| = 2 < 5
		},
		{
			name:     "Percent Step - Anomaly",
			val:      20.0,
			baseline: []float32{10.0, 10.0, 10.0},
			algo:     types.PercentStep,
			interest: 0.5,  // 50%
			want:     true, // (20-10)/10 = 1.0 >= 0.5
		},
		{
			name:     "Percent Step - Normal",
			val:      11.0,
			baseline: []float32{10.0, 10.0, 10.0},
			algo:     types.PercentStep,
			interest: 0.5,
			want:     false, // (11-10)/10 = 0.1 < 0.5
		},
		{
			name:     "Cohen Step - Anomaly",
			val:      100.0,
			baseline: []float32{10.0, 10.1, 9.9}, // Mean ~10, StdDev small
			algo:     types.CohenStep,
			interest: 2.0,
			want:     true, // Huge jump
		},
		{
			name:     "Cohen Step - Normal",
			val:      10.05,
			baseline: []float32{10.0, 10.1, 9.9},
			algo:     types.CohenStep,
			interest: 2.0,
			want:     false,
		},
		{
			name:     "Const Step - Anomaly",
			val:      10.0,
			baseline: []float32{0.0, 0.0, 0.0}, // Baseline doesn't matter for Const
			algo:     types.Const,
			interest: 5.0,
			want:     true, // 10 >= 5
		},
		{
			name:     "Unknown Algo - Defaults to Cohen",
			val:      100.0,
			baseline: []float32{10.0, 10.1, 9.9},
			algo:     types.OriginalStep,
			interest: 50.0, // Should be ignored and use defaultCohenDThreshold (2.0)
			want:     true, // Huge jump
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isAnomaly(tc.val, tc.baseline, string(tc.algo), tc.interest, stdDevThreshold)
			assert.Equal(t, tc.want, got)
		})
	}
}
