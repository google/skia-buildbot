package main

import (
	"crypto/md5"
	"encoding/hex"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
	"math"
	"math/rand"
)

type randomTestSettings struct {
	Corpus               string
	TestName             string
	NumCommits           int
	MinAdditionalKeys    int
	MaxAdditionalKeys    int
	MinAdditionalOptions int
	MaxAdditionalOptions int
	MinTraceDensity      float32
	MaxTraceDensity      float32
	NumTraces            int
}

func generateTracesForTest(sng randomTestSettings) []tiling.TracePair {
	seenIDs := map[tiling.TraceID]bool{}
	rv := make([]tiling.TracePair, sng.NumTraces)
	for i := range rv {
		rKeys := randomKeys(sng.MinAdditionalKeys, sng.MaxAdditionalKeys)
		id := tiling.TraceIDFromParams(rKeys)
		// Make sure we don't generate the same trace twice in a given test.
		for seenIDs[id] {
			rKeys = randomKeys(sng.MinAdditionalKeys, sng.MaxAdditionalKeys)
			id = tiling.TraceIDFromParams(rKeys)
		}
		seenIDs[id] = true

		rOpts := randomOpts(sng.MinAdditionalOptions, sng.MaxAdditionalOptions)
		rKeys[types.CorpusField] = sng.Corpus
		rKeys[types.PrimaryKeyField] = sng.TestName
		t := tiling.NewEmptyTrace(sng.NumCommits, rKeys, rOpts)
		rv[i].Trace = t
		rv[i].ID = id
	}

	// This represents the maximum number of digests that could be seen in this test. It is not
	// certain because if this number is large, the traces could maybe not sample all of them.
	numUniqueDigests := drawRandomDigestsForTest()

	testWideDigests := make([]types.Digest, numUniqueDigests)
	for i := range testWideDigests {
		testWideDigests[i] = randomDigest(sng.TestName)
	}

	digestsPerTrace := make([]types.DigestSlice, sng.NumTraces)
	// Assign the digests for each trace. Each trace will have a flakiness rating assigned
	// randomly simulating real-world data.
	for i := range rv {
		flakiness := drawRandomFlakinessRating(sng.NumCommits)
		if flakiness > len(testWideDigests) {
			flakiness = len(testWideDigests)
		}
		// Shuffle the main list of digests and copy off the first n digests.
		digests := make(types.DigestSlice, flakiness)
		rand.Shuffle(len(testWideDigests), func(i, j int) {
			testWideDigests[i], testWideDigests[j] = testWideDigests[j], testWideDigests[i]
		})
		// We need to copy because a subslice is backed by the original data buffer, which will
		// be shuffled on future iterations.
		copy(digests, testWideDigests[:flakiness])
		digestsPerTrace[i] = digests
	}

	// The easiest way to put a known number of digests into position w/o overwriting previous
	// placements is to shuffle the indexes, so we know there are no duplicates.
	idxs := make([]int, sng.NumCommits)
	for i := range idxs {
		idxs[i] = i
	}

	// Fill out each trace accordingly
	for i := range rv {
		trace := rv[i].Trace
		digests := digestsPerTrace[i]
		// Shuffle the indexes
		rand.Shuffle(len(idxs), func(i, j int) {
			idxs[i], idxs[j] = idxs[j], idxs[i]
		})
		// Be sure to store each digest at least once.
		for j := range digests {
			// Store the jth digest into the jth randomized index of the digests
			trace.Digests[idxs[j]] = digests[j]
		}
		// Randomly fill out the remaining indexes until we reach the randomly chosen density.
		density := rf(sng.MinTraceDensity, sng.MaxTraceDensity)
		for j := len(digests); j < int(density*float32(sng.NumCommits)); j++ {
			trace.Digests[idxs[j]] = digests[rand.Intn(len(digests))]
		}
	}

	return rv
}

// drawRandomDigestsForTest returns a positive random integer that corresponds to the distribution
// of digests per test observed in the Skia data as of November 2020.
// It was observed that the median number of digests/test was 57, the 90th percentile was 550 and
// the 99th percentile was 5500. This somewhat heavy tailed distribution can be approximated by a
// piecewise function. 50% of the time, a random number between 1 and 50 is selected. The other 50%
// of the time, a Pareto distribution with x_m = 50 and a = 0.75 is used. An absolute max of 10000
// is enforced. This distribution was found by messing around with numbers until they looked
// "about right" in a spreadsheet. A normal distribution was not "heavy tailed" enough.
func drawRandomDigestsForTest() int {
	piece := rand.Float32()
	if piece < 0.5 {
		return r(1, 50)
	}
	return int(math.Min(10000, math.Round(randomParetoFromDistribution(50, 0.75))))
}

// drawRandomFlakinessRating returns a positive random integer that corresponds to the
// distribution of unique digests per trace observed in the Skia data as of November 2020.
// Out of the 1.4 million traces, 190k (~14%) had 2 or more unique digests seen in the previous
// 256 commits. 3 or more is the 97th percentile, 4 or more is 99.5th percentile. This can be
// approximated by a Pareto distribution with x_m = 1 and a = 2.75. The one caveat is that starting
// at 10 unique digests, the Pareto distribution converges too fast (not heavy tailed enough).
// Since 10 corresponds to the 99.9th percentile, 1 in 1000 times, we return a random number
// between 11 and max to indicate a "super flaky" trace.
func drawRandomFlakinessRating(max int) int {
	piece := rand.Float32()
	if piece < 0.999 {
		return int(math.Min(float64(max), math.Round(randomParetoFromDistribution(1, 2.75))))
	}
	return r(11, max)
}

func randomParetoFromDistribution(xm, alpha float64) float64 {
	// Based off https://github.com/gonum/gonum/blob/master/stat/distuv/pareto.go
	return math.Exp(math.Log(xm) + 1/alpha*rand.ExpFloat64())
}

// randomDigest returns a digest whose first 8 bytes (16 hex letters) are random,
// and the remaining 8 bytes (16 hex letters) are dependent on the name. This way, we can tell
// which digests are supposed to belong to the same test and which are not, while still maintaining
// a roughly even set of prefixes (CockroachDB sorts things by prefix).
func randomDigest(name string) types.Digest {
	digest := md5.Sum([]byte(name))
	// overwrite the first 8 bytes of the hash randomly
	_, _ = rand.Read(digest[:md5.Size/2]) // always nil error
	return types.Digest(hex.EncodeToString(digest[:]))
}

func randomKeys(min, max int) map[string]string {
	num := r(min, max)
	rv := make(map[string]string, num+2) // add 2 because we'll be adding test and corpus to it.
	for len(rv) < num {
		n := rand.Intn(len(words))
		k := words[n]
		v := words[rand.Intn(n+1)] // some keys have a few variations, others have a lot.
		rv[k] = k + "_" + v
	}
	return rv
}

func randomOpts(min, max int) map[string]string {
	num := r(min, max)
	rv := make(map[string]string, num)
	for len(rv) < num {
		n := rand.Intn(len(words))
		k := words[n]
		v := words[rand.Intn(5)] // options have one of 5 values
		rv["opt_"+k] = "opt_" + k + "_" + v
	}
	return rv
}

// r returns a random integer between [min, max).
func r(min, max int) int {
	return rand.Intn(max-min) + min
}

// rf returns a random float32 between [min, max).
func rf(min, max float32) float32 {
	return rand.Float32()*(max-min) + min
}

var words = []string{
	"Alabama",
	"Alaska",
	"Arizona",
	"Arkansas",
	"California",
	"Colorado",
	"Connecticut",
	"Delaware",
	"Florida",
	"Georgia",
	"Hawaii",
	"Idaho",
	"Illinois",
	"Indiana",
	"Iowa",
	"Kansas",
	"Kentucky",
	"Louisiana",
	"Maine",
	"Maryland",
	"Massachusetts",
	"Michigan",
	"Minnesota",
	"Mississippi",
	"Missouri",
	"Montana",
	"Nebraska",
	"Nevada",
	"New_Hampshire",
	"New_Jersey",
	"New_Mexico",
	"New_York",
	"North_Carolina",
	"North_Dakota",
	"Ohio",
	"Oklahoma",
	"Oregon",
	"Pennsylvania",
	"Rhode_Island",
	"South_Carolina",
	"South_Dakota",
	"Tennessee",
	"Texas",
	"Utah",
	"Vermont",
	"Virginia",
	"Washington",
	"West_Virginia",
	"Wisconsin",
	"Wyoming",
}
