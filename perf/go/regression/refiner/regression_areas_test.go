// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package refiner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/types"
)

func TestFindRegressionAreas(t *testing.T) {
	// Helper to reduce boilerplate in test cases
	u := func(i int) *regression.RegressionDetectionResponse {
		return createResponse(i, "t1", stepfit.UNINTERESTING)
	}
	l := func(i int) *regression.RegressionDetectionResponse { return createResponse(i, "t1", stepfit.LOW) }
	h := func(i int) *regression.RegressionDetectionResponse { return createResponse(i, "t1", stepfit.HIGH) }

	tests := []struct {
		name      string
		input     []*regression.RegressionDetectionResponse
		direction alerts.Direction
		radius    int
		step      types.StepDetection
		expected  int // Number of groups found
	}{
		{
			name:      "Empty input",
			input:     []*regression.RegressionDetectionResponse{},
			direction: alerts.BOTH,
			expected:  0,
		},
		{
			name:      "Uninteresting only",
			input:     []*regression.RegressionDetectionResponse{u(100), u(101)},
			direction: alerts.BOTH,
			expected:  0,
		},
		{
			name:      "Low only (no boundary)",
			input:     []*regression.RegressionDetectionResponse{l(100)},
			direction: alerts.BOTH,
			expected:  1,
		},
		{
			name:      "Low with Left Boundary",
			input:     []*regression.RegressionDetectionResponse{u(99), l(100)},
			direction: alerts.BOTH,
			expected:  0, // Missing Right Boundary
		},
		{
			name:      "Low with Right Boundary",
			input:     []*regression.RegressionDetectionResponse{l(100), u(101)},
			direction: alerts.BOTH,
			expected:  0, // Missing Left Boundary
		},
		{
			name:      "Two Lows with Boundary",
			input:     []*regression.RegressionDetectionResponse{u(99), l(100), l(105), u(106)},
			direction: alerts.BOTH,
			expected:  1, // Should be 1 group containing both LOWs (since we removed adjacent check)
		},
		{
			name:      "Low then High with Boundary",
			input:     []*regression.RegressionDetectionResponse{u(99), l(100), h(101), u(102)},
			direction: alerts.BOTH,
			expected:  2, // L group has U, H. H group has L, U. Valid.
		},
		{
			name: "Low then High (No Boundary between)",
			// U, L, H -> L has U, H (Valid). H has L, End (Invalid).
			input:     []*regression.RegressionDetectionResponse{u(99), l(100), h(101)},
			direction: alerts.BOTH,
			expected:  1,
		},
		{
			name: "High then Low (No Boundary between)",
			// H, L, U -> H has Start, L (Invalid). L has H, U (Valid).
			input:     []*regression.RegressionDetectionResponse{h(100), l(101), u(102)},
			direction: alerts.BOTH,
			expected:  1,
		},
		{
			name:      "Mixed Stream",
			input:     []*regression.RegressionDetectionResponse{u(10), l(11), l(12), u(13), h(14), h(15), u(16)},
			direction: alerts.BOTH,
			expected:  2, // L group has U, U. H group has U, U. Valid.
		},
		{
			name:      "Low Then High",
			input:     []*regression.RegressionDetectionResponse{l(1), h(2)},
			direction: alerts.BOTH,
			expected:  2, // All interesting -> 2 groups
		},
		// Direction filtering tests
		{
			name:      "Direction UP (Ignore Low)",
			input:     []*regression.RegressionDetectionResponse{u(99), l(100), u(101)},
			direction: alerts.UP,
			expected:  0, // Low is treated as uninteresting
		},
		{
			name:      "Direction UP (Keep High)",
			input:     []*regression.RegressionDetectionResponse{u(99), h(100), u(101)},
			direction: alerts.UP,
			expected:  1,
		},
		{
			name:      "Direction DOWN (Ignore High)",
			input:     []*regression.RegressionDetectionResponse{u(99), h(100), u(101)},
			direction: alerts.DOWN,
			expected:  0, // High is treated as uninteresting
		},
		{
			name:      "Direction DOWN (Keep Low)",
			input:     []*regression.RegressionDetectionResponse{u(99), l(100), u(101)},
			direction: alerts.DOWN,
			expected:  1,
		},
		{
			name:      "Direction DOWN (Keep Low)",
			input:     []*regression.RegressionDetectionResponse{h(99), l(100), u(101)},
			direction: alerts.DOWN,
			expected:  1,
		},
		{
			name:      "Direction DOWN (Keep Low) v2",
			input:     []*regression.RegressionDetectionResponse{h(99), l(100), l(101), h(102)},
			direction: alerts.DOWN,
			expected:  1,
		},
		// Splitting tests
		{
			name: "Do Not Split Large Area (len=4, radius=2 -> 4 is not > 2*2)",
			// u, h, h, h, h, u -> area is h,h,h,h (len 4). Radius 2.
			// 4 is NOT > 4 -> no split.
			input:     []*regression.RegressionDetectionResponse{u(9), h(10), h(11), h(12), h(13), u(14)},
			direction: alerts.BOTH,
			radius:    2,
			expected:  1, // No split
		},
		{
			name: "Split Large Area (len=5, radius=2 -> split 2,3)",
			// u, h, h, h, h, h, u -> area len 5. Radius 2.
			// 5 >= 4 -> split. 2, 3.
			input:     []*regression.RegressionDetectionResponse{u(9), h(10), h(11), h(12), h(13), h(14), u(15)},
			direction: alerts.BOTH,
			radius:    2,
			expected:  2,
		},
		{
			name: "Do Not Split (len=3, radius=2 -> < 2*R)",
			// u, h, h, h, u -> area len 3. Radius 2.
			// 3 < 4 -> no split.
			input:     []*regression.RegressionDetectionResponse{u(9), h(10), h(11), h(12), u(13)},
			direction: alerts.BOTH,
			radius:    2,
			expected:  1,
		},
		{
			name: "Split Large Area + 1 range without split",
			// u, h, h, h, h, h, u -> area len 5. Radius 2.
			// 5 >= 4 -> split. 2, 3.
			input:     []*regression.RegressionDetectionResponse{u(9), h(10), h(11), h(12), h(13), h(14), u(15), h(16), u(17)},
			direction: alerts.BOTH,
			radius:    2,
			expected:  3,
		},
		{
			name: "All Interesting (Edge Case)",
			// All are HIGH. BOTH direction.
			// Should return 3 groups, each size 1.
			input:     []*regression.RegressionDetectionResponse{h(10), h(11), h(12)},
			direction: alerts.BOTH,
			expected:  3,
		},
		{
			name: "All Interesting (Edge Case) 2",
			// All are HIGH. BOTH direction.
			// Should return 3 groups, each size 1.
			input:     []*regression.RegressionDetectionResponse{h(10), h(11), h(12), l(13), l(14), l(15), h(16), h(17), h(18)},
			direction: alerts.DOWN,
			expected:  1,
		},
		{
			name: "All Interesting with Const step (Group 1 Log Skipped)",
			// All are HIGH. BOTH direction. Const step.
			input:     []*regression.RegressionDetectionResponse{h(10), h(11)},
			direction: alerts.BOTH,
			step:      types.Const,
			expected:  2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.radius == 0 {
				tc.radius = 1000 // Default large radius to avoid splitting in non-splitting tests
			}
			cfg := &alerts.Alert{DirectionAsString: tc.direction, Radius: tc.radius, Step: tc.step}
			areas := findRegressionAreas(tc.input, cfg)
			assert.Len(t, areas, tc.expected)

			// Optional: Verify content if needed, but count is good proxy for grouping logic correctness here.
			if tc.expected > 0 {
				for _, area := range areas {
					assert.NotEmpty(t, area)
					firstStatus := area[0].Summary.Clusters[0].StepFit.Status
					for _, item := range area {
						assert.Equal(t, firstStatus, item.Summary.Clusters[0].StepFit.Status, "Group should have consistent status")
					}
				}
			}
		})
	}
}

func TestFindRegressionAreas_Content(t *testing.T) {
	u := func(i int) *regression.RegressionDetectionResponse {
		return createResponse(i, "t1", stepfit.UNINTERESTING)
	}
	h := func(i int) *regression.RegressionDetectionResponse { return createResponse(i, "t1", stepfit.HIGH) }

	// Input: u(9), h(10), h(11), h(12), h(13), h(14), h(15), h(16), h(17), h(18), u(19), h(20), u(21)
	// Block 1: h(10)..h(18) (Len 9). Radius 2.
	// Split:
	//   Chunk 1: 10 - 13 (4 items)
	//   Chunk 2: 14 - 17 (4 items)
	//   Chunk 3: 18 (1 item)
	// Block 2: h(20) (Len 1). No split.
	input := []*regression.RegressionDetectionResponse{u(9), h(10), h(11), h(12), h(13), h(14), h(15), h(16), h(17), h(18), u(19), h(20), u(21)}
	cfg := &alerts.Alert{DirectionAsString: alerts.BOTH, Radius: 2}

	areas := findRegressionAreas(input, cfg)
	assert.Len(t, areas, 4)

	// Verify Group 1: 10 - 13
	assert.Len(t, areas[0], 4)
	assert.Equal(t, 10, int(getOffset(areas[0][0])))
	assert.Equal(t, 13, int(getOffset(areas[0][3])))

	// Verify Group 2: 14 - 17
	assert.Len(t, areas[1], 4)
	assert.Equal(t, 14, int(getOffset(areas[1][0])))
	assert.Equal(t, 17, int(getOffset(areas[1][3])))

	// Verify Group 3: 18
	assert.Len(t, areas[2], 1)
	assert.Equal(t, 18, int(getOffset(areas[2][0])))

	// Verify Group 4: 20
	assert.Len(t, areas[3], 1)
	assert.Equal(t, 20, int(getOffset(areas[3][0])))
}

func getOffset(r *regression.RegressionDetectionResponse) types.CommitNumber {
	return r.Summary.Clusters[0].StepPoint.Offset
}
