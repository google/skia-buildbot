// This package supplies test data that matches the following scenario:
// There are two tests run by each of four devices. On the second commit,
// a developer introduces a bug, causing both tests to start drawing
// untriaged images. Then, on the fourth commit, the bug was reverted,
// restoring the expected images.
package data_bug_revert

import (
	"time"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

const (
	// These digests are valid, but arbitrary
	GoodDigestAlfa         = types.Digest("aaa0ddfc45a95372747804fc75061fc1")
	GoodDigestCharlie      = types.Digest("ccc609f89667947852479995dc3b625e")
	GoodDigestEcho         = types.Digest("eeec3fc301ef00a9c193f9c5abd664ba")
	UntriagedDigestBravo   = types.Digest("bbbaac428fc2fc1f4dd3707031a6dc6d")
	UntriagedDigestDelta   = types.Digest("ddd3199bda4b909d3ba2ab14120998cd")
	UntriagedDigestFoxtrot = types.Digest("fff13007fba3a6edd4d600eb891286ca")
	// Less typing below
	missingDigest = types.MISSING_DIGEST

	TestOne = types.TestName("test_one")
	TestTwo = types.TestName("test_two")

	AlphaDevice = "alpha"
	BetaDevice  = "beta"
	GammaDevice = "gamma"
	DeltaDevice = "delta"

	InnocentAuthor = "innocent@example.com"
	BuggyAuthor    = "buggy@example.com"

	FirstCommitHash  = "1ea258b693f2fc53501ac341f3029860b3b57a10"
	SecondCommitHash = "22ac03f867b38dfa488c57030af3663bcaae3736"
	ThirdCommitHash  = "331432b919fab5ca878757fff4766cc12936f82c"
	FourthCommitHash = "437c7001b3cd9e81d4d67bbaf8816e00b29a7dd4"
	FifthCommitHash  = "5f8a8418769962eddd02c36d52f3ab3b775f926a"
)

func MakeTestCommits() []*tiling.Commit {
	// Five commits, with completely arbitrary data
	return []*tiling.Commit{
		{
			Hash:       FirstCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     InnocentAuthor,
		},
		{
			Hash:       SecondCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 12, 10, 18, 0, time.UTC).Unix(),
			Author:     BuggyAuthor,
		},
		{
			Hash:       ThirdCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     InnocentAuthor,
		},
		{
			Hash:       FourthCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 15, 28, 0, time.UTC).Unix(),
			Author:     BuggyAuthor,
		},
		{
			Hash:       FifthCommitHash,
			CommitTime: time.Date(2019, time.May, 26, 13, 35, 38, 0, time.UTC).Unix(),
			Author:     InnocentAuthor,
		},
	}
}

func MakeTestTile() *tiling.Tile {
	return &tiling.Tile{
		Commits:   MakeTestCommits(),
		Scale:     0, // tile contains every data point.
		TileIndex: 0,

		Traces: map[tiling.TraceId]tiling.Trace{
			",device=alpha,name=test_one,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					// A very clear history showing 2nd commit as the change to bravo
					// The next three traces are the same data, just with various bits missing.
					GoodDigestAlfa, UntriagedDigestBravo, UntriagedDigestBravo, GoodDigestAlfa, GoodDigestAlfa,
				},
				Keys: map[string]string{
					"device":                AlphaDevice,
					types.PRIMARY_KEY_FIELD: string(TestOne),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=beta,name=test_one,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					GoodDigestAlfa, missingDigest, UntriagedDigestBravo, missingDigest, GoodDigestAlfa,
				},
				Keys: map[string]string{
					"device":                BetaDevice,
					types.PRIMARY_KEY_FIELD: string(TestOne),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=gamma,name=test_one,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					GoodDigestAlfa, UntriagedDigestBravo, missingDigest, missingDigest, GoodDigestAlfa,
				},
				Keys: map[string]string{
					"device":                GammaDevice,
					types.PRIMARY_KEY_FIELD: string(TestOne),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=delta,name=test_one,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					missingDigest, UntriagedDigestBravo, missingDigest, missingDigest, GoodDigestAlfa,
				},
				Keys: map[string]string{
					"device":                DeltaDevice,
					types.PRIMARY_KEY_FIELD: string(TestOne),
					types.CORPUS_FIELD:      "gm",
				},
			},

			",device=alpha,name=test_two,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					// A very clear history showing 2nd commit as the change to bravo
					// The next trace is the same data, just with various bits missing.
					GoodDigestCharlie, UntriagedDigestDelta, UntriagedDigestDelta, GoodDigestCharlie, GoodDigestCharlie,
				},
				Keys: map[string]string{
					"device":                AlphaDevice,
					types.PRIMARY_KEY_FIELD: string(TestTwo),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=beta,name=test_two,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					GoodDigestCharlie, missingDigest, missingDigest, missingDigest, GoodDigestCharlie,
				},
				Keys: map[string]string{
					"device":                BetaDevice,
					types.PRIMARY_KEY_FIELD: string(TestTwo),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=gamma,name=test_two,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					// A somewhat flaky trace, using multiple positive/untriaged digests.
					GoodDigestCharlie, UntriagedDigestDelta, UntriagedDigestFoxtrot, missingDigest, GoodDigestEcho,
				},
				Keys: map[string]string{
					"device":                GammaDevice,
					types.PRIMARY_KEY_FIELD: string(TestTwo),
					types.CORPUS_FIELD:      "gm",
				},
			},
			",device=delta,name=test_two,source_type=gm,": &types.GoldenTrace{
				Digests: types.DigestSlice{
					// Here's an interesting case where the culprit isn't accurately identified
					// due to missing data. Here, both the authors of the 2nd and 3rd commit
					// are possibly to blame.
					GoodDigestEcho, missingDigest, UntriagedDigestFoxtrot, missingDigest, missingDigest,
				},
				Keys: map[string]string{
					"device":                DeltaDevice,
					types.PRIMARY_KEY_FIELD: string(TestTwo),
					types.CORPUS_FIELD:      "gm",
				},
			},
		},

		// Summarizes all the keys and values seen in this tile
		// The values should be in alphabetical order (see paramset.Normalize())
		ParamSet: map[string][]string{
			"device":                {AlphaDevice, BetaDevice, GammaDevice, DeltaDevice},
			types.PRIMARY_KEY_FIELD: {string(TestOne), string(TestTwo)},
			types.CORPUS_FIELD:      {"gm"},
		},
	}
}

func MakeTestExpectations() *expectations.Expectations {
	var e expectations.Expectations
	e.Set(TestOne, GoodDigestAlfa, expectations.Positive)
	e.Set(TestTwo, GoodDigestCharlie, expectations.Positive)
	e.Set(TestTwo, GoodDigestEcho, expectations.Positive)
	return &e
}
