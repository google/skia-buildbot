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

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

// human-readable variable names for the data (values are arbitrary, but valid)
const (
	AlphaPositiveDigest  = types.Digest("000075b9c0f1b6a831c399e269772661")
	AlphaNegativeDigest  = types.Digest("11115ffee6ae2fec3ad71c777531578f")
	AlphaUntriagedDigest = types.Digest("222208f09d37b73795649038408b5f33")

	BetaPositiveDigest  = types.Digest("4444e0910d750195b448797616e091ad")
	BetaUntriagedDigest = types.Digest("55554cdd754f91cc6554c9e71929cce7")

	FirstCommitHash  = "aaaaad283f72b5d51ecada8ec56ec8ff4aa81c6c"
	SecondCommitHash = "bbbbb829a2384b001cc12b0c2613c756454a1f6a"
	ThirdCommitHash  = "cccccdf52094181356d60845ee5cf1d83aec6d2a"

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
	e := MakeTestExpectations()
	b := baseline.Baseline{
		Expectations:     e.AsBaseline(),
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

		Traces: map[tiling.TraceID]*tiling.GoldenTrace{
			AnglerAlphaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{AlphaNegativeDigest, AlphaNegativeDigest, AlphaPositiveDigest},
				map[string]string{
					"device":              AnglerDevice,
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "gm",
				},
			),
			AnglerBetaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{BetaPositiveDigest, BetaPositiveDigest, BetaPositiveDigest},
				map[string]string{
					"device":              AnglerDevice,
					types.PrimaryKeyField: string(BetaTest),
					types.CorpusField:     "gm",
				},
			),

			BullheadAlphaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{AlphaNegativeDigest, AlphaNegativeDigest, AlphaUntriagedDigest},
				map[string]string{
					"device":              BullheadDevice,
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "gm",
				},
			),
			BullheadBetaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{BetaPositiveDigest, BetaPositiveDigest, BetaPositiveDigest},
				map[string]string{
					"device":              BullheadDevice,
					types.PrimaryKeyField: string(BetaTest),
					types.CorpusField:     "gm",
				},
			),

			CrosshatchAlphaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{AlphaNegativeDigest, AlphaNegativeDigest, AlphaPositiveDigest},
				map[string]string{
					"device":              CrosshatchDevice,
					types.PrimaryKeyField: string(AlphaTest),
					types.CorpusField:     "gm",
				},
			),
			CrosshatchBetaTraceID: tiling.NewGoldenTrace(
				types.DigestSlice{BetaUntriagedDigest, tiling.MissingDigest, tiling.MissingDigest},
				map[string]string{
					"device":              CrosshatchDevice,
					types.PrimaryKeyField: string(BetaTest),
					types.CorpusField:     "gm",
				},
			),
		},

		// Summarizes all the keys and values seen in this tile
		// The values should be in alphabetical order (see paramset.Normalize())
		ParamSet: map[string][]string{
			"device":              {AnglerDevice, BullheadDevice, CrosshatchDevice},
			types.PrimaryKeyField: {string(AlphaTest), string(BetaTest)},
			types.CorpusField:     {"gm"},
		},
	}
}

func MakeTestExpectations() *expectations.Expectations {
	var e expectations.Expectations
	e.Set(AlphaTest, AlphaPositiveDigest, expectations.Positive)
	e.Set(AlphaTest, AlphaUntriagedDigest, expectations.Untriaged)
	e.Set(AlphaTest, AlphaNegativeDigest, expectations.Negative)

	e.Set(BetaTest, BetaPositiveDigest, expectations.Positive)
	e.Set(BetaTest, BetaUntriagedDigest, expectations.Untriaged)
	return &e
}
