package stats

import "go.skia.org/infra/golden/go/types"

// Given a tile, produce a tile that only contains one test.

type Summary struct{}

func CalcTestSummary(testName, corpus string, traces []*types.GoldenTrace) (*Summary, error) {
	return nil, nil
}
