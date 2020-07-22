// Package data_bug_revert supplies test data that matches the following scenario:
// There are two tests run by each of four devices. On the second commit,
// a developer introduces a bug, causing both tests to start drawing
// untriaged images. Then, on the fourth commit, the bug was reverted,
// restoring the expected images.
package data_bug_revert

import (
	"time"

	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
)

const (
	// These digests are valid, but arbitrary
	AlfaPositiveDigest     = types.Digest("aaa0ddfc45a95372747804fc75061fc1")
	CharliePositiveDigest  = types.Digest("ccc609f89667947852479995dc3b625e")
	EchoPositiveDigest     = types.Digest("eeec3fc301ef00a9c193f9c5abd664ba")
	BravoUntriagedDigest   = types.Digest("bbbaac428fc2fc1f4dd3707031a6dc6d")
	DeltaUntriagedDigest   = types.Digest("ddd3199bda4b909d3ba2ab14120998cd")
	FoxtrotUntriagedDigest = types.Digest("fff13007fba3a6edd4d600eb891286ca")
	// Less typing below
	missingDigest = tiling.MissingDigest

	TestOne = types.TestName("test_one")
	TestTwo = types.TestName("test_two")

	AlphaDevice = "alpha"
	BetaDevice  = "beta"
	GammaDevice = "gamma"
	DeltaDevice = "delta"

	InnocentAuthor = "innocent@example.com"
	BuggyAuthor    = "buggy@example.com"

	FirstCommitHash  = "111118b693f2fc53501ac341f3029860b3b57a10"
	SecondCommitHash = "222223f867b38dfa488c57030af3663bcaae3736"
	ThirdCommitHash  = "333332b919fab5ca878757fff4766cc12936f82c"
	FourthCommitHash = "44444001b3cd9e81d4d67bbaf8816e00b29a7dd4"
	FifthCommitHash  = "55555418769962eddd02c36d52f3ab3b775f926a"

	BugIntroducedCommitIndex = 2
	RevertBugCommitIndex     = 4
)

func MakeTestCommits() []tiling.Commit {
	// Five commits, with completely arbitrary data
	return []tiling.Commit{
		{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 12, 0, 3, 0, time.UTC),
			Author:     InnocentAuthor,
			Subject:    "Just an ordinary commit",
		},
		{
			Hash:       SecondCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 12, 10, 18, 0, time.UTC),
			Author:     BuggyAuthor,
			Subject:    "I hope this doesn't have a bug",
		},
		{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 10, 8, 0, time.UTC),
			Author:     InnocentAuthor,
			Subject:    "whitespace change",
		},
		{
			Hash:       FourthCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 15, 28, 0, time.UTC),
			Author:     BuggyAuthor,
			Subject:    "revert 'I hope this doesn't have a bug'",
		},
		{
			Hash:       FifthCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 35, 38, 0, time.UTC),
			Author:     InnocentAuthor,
			Subject:    "documentation change",
		},
	}
}

func MakeTestTile() *tiling.Tile {
	return &tiling.Tile{
		Commits: MakeTestCommits(),
		Traces: map[tiling.TraceID]*tiling.Trace{
			",device=alpha,name=test_one,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				// A very clear history showing 2nd commit as the change to bravo
				// The next three traces are the same data, just with various bits missing.
				AlfaPositiveDigest, BravoUntriagedDigest, BravoUntriagedDigest, AlfaPositiveDigest, AlfaPositiveDigest,
			}, map[string]string{
				"device":              AlphaDevice,
				types.PrimaryKeyField: string(TestOne),
				types.CorpusField:     "gm",
			}, nil),
			",device=beta,name=test_one,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				AlfaPositiveDigest, missingDigest, BravoUntriagedDigest, missingDigest, AlfaPositiveDigest,
			}, map[string]string{
				"device":              BetaDevice,
				types.PrimaryKeyField: string(TestOne),
				types.CorpusField:     "gm",
			}, nil),
			",device=gamma,name=test_one,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				AlfaPositiveDigest, BravoUntriagedDigest, missingDigest, missingDigest, AlfaPositiveDigest,
			}, map[string]string{
				"device":              GammaDevice,
				types.PrimaryKeyField: string(TestOne),
				types.CorpusField:     "gm",
			}, nil),
			",device=delta,name=test_one,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				missingDigest, BravoUntriagedDigest, missingDigest, missingDigest, AlfaPositiveDigest,
			}, map[string]string{
				"device":              DeltaDevice,
				types.PrimaryKeyField: string(TestOne),
				types.CorpusField:     "gm",
			}, nil),

			",device=alpha,name=test_two,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				// A very clear history showing 2nd commit as the change to bravo
				// The next trace is the same data, just with various bits missing.
				CharliePositiveDigest, DeltaUntriagedDigest, DeltaUntriagedDigest, CharliePositiveDigest, CharliePositiveDigest,
			}, map[string]string{
				"device":              AlphaDevice,
				types.PrimaryKeyField: string(TestTwo),
				types.CorpusField:     "gm",
			}, nil),
			",device=beta,name=test_two,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				CharliePositiveDigest, missingDigest, missingDigest, missingDigest, CharliePositiveDigest,
			}, map[string]string{
				"device":              BetaDevice,
				types.PrimaryKeyField: string(TestTwo),
				types.CorpusField:     "gm",
			}, nil),
			",device=gamma,name=test_two,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				// A somewhat flaky trace, using multiple positive/untriaged digests.
				CharliePositiveDigest, DeltaUntriagedDigest, FoxtrotUntriagedDigest, missingDigest, EchoPositiveDigest,
			}, map[string]string{
				"device":              GammaDevice,
				types.PrimaryKeyField: string(TestTwo),
				types.CorpusField:     "gm",
			}, nil),
			",device=delta,name=test_two,source_type=gm,": tiling.NewTrace(types.DigestSlice{
				// Here's an interesting case where the culprit isn't accurately identified
				// due to missing data. Here, both the authors of the 2nd and 3rd commit
				// are possibly to blame.
				EchoPositiveDigest, missingDigest, FoxtrotUntriagedDigest, missingDigest, missingDigest,
			}, map[string]string{
				"device":              DeltaDevice,
				types.PrimaryKeyField: string(TestTwo),
				types.CorpusField:     "gm",
			}, nil),
		},

		// Summarizes all the keys and values seen in this tile
		// The values should be in alphabetical order (see paramset.Normalize())
		ParamSet: map[string][]string{
			"device":              {AlphaDevice, BetaDevice, GammaDevice, DeltaDevice},
			types.PrimaryKeyField: {string(TestOne), string(TestTwo)},
			types.CorpusField:     {"gm"},
		},
	}
}

func MakeTestExpectations() *expectations.Expectations {
	var e expectations.Expectations
	e.Set(TestOne, AlfaPositiveDigest, expectations.Positive)
	e.Set(TestTwo, CharliePositiveDigest, expectations.Positive)
	e.Set(TestTwo, EchoPositiveDigest, expectations.Positive)
	return &e
}
