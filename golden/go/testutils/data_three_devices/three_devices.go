package data_three_devices

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
)

// This package supplies test data that matches the following scenario:
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

	AlphaTest = types.TestName("test_alpha")
	BetaTest  = types.TestName("test_beta")
)

func MakeTestBaseline() *baseline.CommitableBaseline {
	b := baseline.CommitableBaseline{
		StartCommit: &tiling.Commit{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     FirstCommitAuthor,
		},
		EndCommit: &tiling.Commit{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     ThirdCommitAuthor,
		},
		Baseline: types.TestExp{
			AlphaTest: map[types.Digest]types.Label{
				// These hashes are arbitrarily made up and have no real-world meaning.
				AlphaGood1Digest:      types.POSITIVE,
				AlphaUntriaged1Digest: types.UNTRIAGED,
				AlphaBad1Digest:       types.NEGATIVE,
			},
			BetaTest: map[types.Digest]types.Label{
				// These hashes are arbitrarily made up and have no real-world meaning.
				BetaGood1Digest:      types.POSITIVE,
				BetaUntriaged1Digest: types.UNTRIAGED,
			},
		},
		Filled: 2, // two tests had at least one positive digest
		Total:  6,
		Issue:  0, // 0 means master branch, by definition
	}
	var err error
	b.MD5, err = util.MD5Sum(b.Baseline)
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
		Scale:     1,
		TileIndex: 0,

		Traces: map[tiling.TraceId]tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			"angler:test_alpha:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaGood1Digest},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			"angler:test_beta:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{BetaGood1Digest, BetaGood1Digest, BetaGood1Digest},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},

			"bullhead:test_alpha:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaUntriaged1Digest},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			"bullhead:test_beta:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{BetaGood1Digest, BetaGood1Digest, BetaGood1Digest},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},

			"crosshatch:test_alpha:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{AlphaBad1Digest, AlphaBad1Digest, AlphaGood1Digest},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: string(AlphaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
			"crosshatch:test_beta:gm": &types.GoldenTrace{
				Digests: types.DigestSlice{BetaUntriaged1Digest, types.MISSING_DIGEST, types.MISSING_DIGEST},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: string(BetaTest),
					types.CORPUS_FIELD:      "gm",
				},
			},
		},
	}
}
