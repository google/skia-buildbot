// Package data_three_devices supplies test data that matches the following scenario:
// There are 3 devices (angler, bullhead, crosshatch, each running 2 tests (test_alpha, test_beta).
//
// All 3 devices drew test_alpha incorrectly as digest alphaBad1Digest at StartCommit.
// Devices angler and crosshatch drew test_alpha correctly as digest alphaGood1Digest at EndCommit.
// Device bullhead drew test_alpha as digest alphaUntriaged1Digest at EndCommit.
//
// Devices angler and bullhead drew test_beta the same (digest betaGood1Digest)
// and device crosshatch the remaining case betaUntriaged1Digest.
// crosshatch is missing two digests (maybe that test hasn't run yet?)
// The baseline is on the master branch.
//
// These helper functions all return a fresh copy of their objects so that
// tests can mutate them w/o impacting future tests.
package data_three_devices

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

// human-readable variable names for the data (values are arbitrary, but valid)
const (
	AlphaGood1Digest      = types.Digest("0cc175b9c0f1b6a831c399e269772661")
	AlphaBad1Digest       = types.Digest("92eb5ffee6ae2fec3ad71c777531578f")
	AlphaUntriaged1Digest = types.Digest("4a8a08f09d37b73795649038408b5f33")

	BetaGood1Digest      = types.Digest("7277e0910d750195b448797616e091ad")
	BetaUntriaged1Digest = types.Digest("8fa14cdd754f91cc6554c9e71929cce7")

	FirstCommitHash  = "a3f82d283f72b5d51ecada8ec56ec8ff4aa81c6c"
	SecondCommitHash = "b52f7829a2384b001cc12b0c2613c756454a1f6a"
	ThirdCommitHash  = "cd77adf52094181356d60845ee5cf1d83aec6d2a"

	FirstCommitAuthor  = "alpha@example.com"
	SecondCommitAuthor = "beta@example.com"
	ThirdCommitAuthor  = "gamma@example.com"

	// Reminder that the ids for the traces are created using the
	// logic in query.MakeKeyFast

	AnglerAlphaTraceID     = ",device=angler,name=test_alpha,source_type=gm,"
	AnglerBetaTraceID      = ",device=angler,name=test_beta,source_type=gm,"
	BullheadAlphaTraceID   = ",device=bullhead,name=test_alpha,source_type=gm,"
	BullheadBetaTraceID    = ",device=bullhead,name=test_beta,source_type=gm,"
	CrosshatchAlphaTraceID = ",device=crosshatch,name=test_alpha,source_type=gm,"
	CrosshatchBetaTraceID  = ",device=crosshatch,name=test_beta,source_type=gm,"

	AlphaTest = types.TestName("test_alpha")
	BetaTest  = types.TestName("test_beta")

	AnglerDevice     = "angler"
	BullheadDevice   = "bullhead"
	CrosshatchDevice = "crosshatch"
)

func MakeTestBaseline() *baseline.Baseline {
	b := baseline.Baseline{
		Expectations:     MakeTestExpectations().AsBaseline(),
		ChangeListID:     "",
		CodeReviewSystem: "",
	}
	var err error
	b.MD5, err = util.MD5Sum(b.Expectations)
	if err != nil {
		panic(fmt.Sprintf("Error computing MD5 of the baseline: %s", err))
	}
	return &b
}

func MakeTestCommits() []*tiling.Commit {
	// Three commits, with completely arbitrary data
	return []*tiling.Commit{
		{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     FirstCommitAuthor,
		},
		{
			Hash:       SecondCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 10, 18, 0, time.UTC).Unix(),
			Author:     SecondCommitAuthor,
		},
		{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     ThirdCommitAuthor,
		},
	}
}

func MakeTestTile() *tiling.Tile {
	return &tiling.Tile{
		Commits:   MakeTestCommits(),
		Scale:     0, // tile contains every data point.
		TileIndex: 0,

		Traces: map[tiling.TraceId]tiling.Trace{
			AnglerAlphaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaGood1Digest},
				Keys: map[string]string{
					"device":                AnglerDevice,
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			AnglerBetaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{BetaGood1Digest, BetaGood1Digest, BetaGood1Digest},
				Keys: map[string]string{
					"device":                AnglerDevice,
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},

			BullheadAlphaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaUntriaged1Digest},
				Keys: map[string]string{
					"device":                BullheadDevice,
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			BullheadBetaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{BetaGood1Digest, BetaGood1Digest, BetaGood1Digest},
				Keys: map[string]string{
					"device":                BullheadDevice,
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},

			CrosshatchAlphaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaGood1Digest},
				Keys: map[string]string{
					"device":                CrosshatchDevice,
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			CrosshatchBetaTraceID: &types.GoldenTrace{
				Digests: types.DigestSlice{BetaUntriaged1Digest, types.MISSING_DIGEST, types.MISSING_DIGEST},
				Keys: map[string]string{
					"device":                CrosshatchDevice,
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
		},

		// Summarizes all the keys and values seen in this tile
		// The values should be in alphabetical order (see paramset.Normalize())
		ParamSet: map[string][]string{
			"device":                {AnglerDevice, BullheadDevice, CrosshatchDevice},
			types.PRIMARY_KEY_FIELD: {string(AlphaTest), string(BetaTest)},
			types.CORPUS_FIELD:      {"gm"},
		},
	}
}

func MakeTestExpectations() expectations.Expectations {
	return expectations.Expectations{
		AlphaTest: map[types.Digest]expectations.Label{
			AlphaGood1Digest:      expectations.Positive,
			AlphaUntriaged1Digest: expectations.Untriaged,
			AlphaBad1Digest:       expectations.Negative,
		},
		BetaTest: map[types.Digest]expectations.Label{
			BetaGood1Digest:      expectations.Positive,
			BetaUntriaged1Digest: expectations.Untriaged,
		},
	}
}
