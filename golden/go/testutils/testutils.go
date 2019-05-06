package testutils

import (
	"net"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/tiling"
	traceservice "go.skia.org/infra/go/trace/service"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc"
)

// StartTestTraceDBServer starts up a traceDB server for testing. It stores its
// data at the given path and returns the address at which the server is
// listening as the second return value.
// Upon completion the calling test should call the Stop() function of the
// returned server object.
func StartTraceDBTestServer(t sktest.TestingT, traceDBFileName, shareDBDir string) (*grpc.Server, string) {
	traceDBServer, err := traceservice.NewTraceServiceServer(traceDBFileName)
	assert.NoError(t, err)

	lis, err := net.Listen("tcp", "localhost:0")
	assert.NoError(t, err)

	server := grpc.NewServer()
	traceservice.RegisterTraceServiceServer(server, traceDBServer)

	go func() {
		// We ignore the error, because calling the Stop() function always causes
		// an error and we are primarily interested in using this to test other code.
		_ = server.Serve(lis)
	}()

	return server, lis.Addr().String()
}

// This baseline represents the following case: There are 3 devices
// (angler, bullhead, crosshatch, each running 2 tests (test_alpha, test_beta)
//
// All 3 devices drew test_alpha incorrectly as digest alphaBad1Hash at StartCommit.
// Devices angler and crosshatch drew test_alpha correctly as digest alphaGood1Hash at EndCommit.
// Device bullhead drew test_alpha as digest alphaUntriaged1Hash at EndCommit.
//
// Devices angler and bullhead drew test_beta the same (digest betaGood1Hash)
// and device crosshatch the remaining case betaUntriaged1Hash.
// crosshatch is missing two digests (maybe that test hasn't run yet?)
// The baseline is on the master branch.
//
// These helper functions all return a fresh copy of their objects so that
// tests can mutate them w/o impacting future tests.
func MakeTestBaselineThreeDevice() *baseline.CommitableBaseline {
	return &baseline.CommitableBaseline{
		StartCommit: &tiling.Commit{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "alpha@example.com",
		},
		EndCommit: &tiling.Commit{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     "gamma@example.com",
		},
		MD5: "hashOfBaseline",
		Baseline: types.TestExp{
			"test_alpha": map[string]types.Label{
				// These hashes are arbitrarily made up and have no real-world meaning.
				AlphaGood1Hash:      types.POSITIVE,
				AlphaUntriaged1Hash: types.UNTRIAGED,
				AlphaBad1Hash:       types.NEGATIVE,
			},
			"test_beta": map[string]types.Label{
				// These hashes are arbitrarily made up and have no real-world meaning.
				BetaGood1Hash:      types.POSITIVE,
				BetaUntriaged1Hash: types.UNTRIAGED,
			},
		},
		Filled: 2, // two tests had at least one positive digest
		Total:  6,
		Issue:  0, // 0 means master branch, by definition
	}
}

func MakeTestCommitsThreeDevice() []*tiling.Commit {
	// Three commits, with completely arbitrary data
	return []*tiling.Commit{
		{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "alpha@example.com",
		},
		{
			Hash:       SecondCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 10, 18, 0, time.UTC).Unix(),
			Author:     "beta@example.com",
		},
		{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     "gamma@example.com",
		},
	}
}

func MakeTestTileThreeDevice() *tiling.Tile {
	return &tiling.Tile{
		Commits:   MakeTestCommitsThreeDevice(),
		Scale:     1,
		TileIndex: 0,

		Traces: map[string]tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			"angler:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{AlphaBad1Hash, AlphaBad1Hash, AlphaGood1Hash},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"angler:test_beta:gm": &types.GoldenTrace{
				Digests: []string{BetaGood1Hash, BetaGood1Hash, BetaGood1Hash},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},

			"bullhead:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{AlphaBad1Hash, AlphaBad1Hash, AlphaUntriaged1Hash},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"bullhead:test_beta:gm": &types.GoldenTrace{
				Digests: []string{BetaGood1Hash, BetaGood1Hash, BetaGood1Hash},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},

			"crosshatch:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{AlphaBad1Hash, AlphaBad1Hash, AlphaGood1Hash},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"crosshatch:test_beta:gm": &types.GoldenTrace{
				Digests: []string{BetaUntriaged1Hash, types.MISSING_DIGEST, types.MISSING_DIGEST},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},
		},
	}
}

// human-readable variable names for the hashes (values are arbitrary, but valid md5 hashes)
const (
	AlphaGood1Hash      = "0cc175b9c0f1b6a831c399e269772661"
	AlphaBad1Hash       = "92eb5ffee6ae2fec3ad71c777531578f"
	AlphaUntriaged1Hash = "4a8a08f09d37b73795649038408b5f33"

	BetaGood1Hash      = "7277e0910d750195b448797616e091ad"
	BetaUntriaged1Hash = "8fa14cdd754f91cc6554c9e71929cce7"

	FirstCommitHash  = "a3f82d283f72b5d51ecada8ec56ec8ff4aa81c6c"
	SecondCommitHash = "b52f7829a2384b001cc12b0c2613c756454a1f6a"
	ThirdCommitHash  = "cd77adf52094181356d60845ee5cf1d83aec6d2a"
)
