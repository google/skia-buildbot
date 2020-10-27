package main

import (
	"crypto/md5"
	"encoding/hex"
	"math/rand"

	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/types"
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
	MinTraceFlakiness    int
	// MaxTraceFlakiness is the most amount of unique digests in a given trace.
	MaxTraceFlakiness int
	// TraceDigestOverlap is the percent chance that a given digest will occur in multiple traces in
	// this digest.
	TraceDigestOverlap float32
	// GlobalDigestOverlap is the probability that a given digest will appear in this test from
	// the "global set" of digests. This simulates the real-world scenario where multiple tests could
	// draw blank.
	GlobalDigestOverlap float32
}

func generateTracesForTest(sng randomTestSettings) []tiling.TracePair {
	if sng.MinTraceFlakiness == 0 {
		sng.MinTraceFlakiness = 1 // ensure there is at least one non-empty digest per trace.
	}
	rv := make([]tiling.TracePair, sng.NumTraces)
	for i := range rv {
		rKeys := randomKeys(sng.MinAdditionalKeys, sng.MaxAdditionalKeys)
		rOpts := randomOpts(sng.MinAdditionalOptions, sng.MaxAdditionalOptions)
		rKeys[types.CorpusField] = sng.Corpus
		rKeys[types.PrimaryKeyField] = sng.TestName
		t := tiling.NewEmptyTrace(sng.NumCommits, rKeys, rOpts)
		rv[i].Trace = t
		rv[i].ID = tiling.TraceIDFromParams(rKeys)
	}

	var testWideDigests []types.Digest
	digestsPerTrace := make([]types.DigestSet, sng.NumTraces)
	// Assign the digests for each trace
	for i := range rv {
		numDigests := r(sng.MinTraceFlakiness, sng.MaxTraceFlakiness)
		if numDigests > sng.NumCommits {
			numDigests = sng.NumCommits
		}
		digests := make(types.DigestSet, numDigests)
		digestsPerTrace[i] = digests
		for len(digests) < numDigests {
			rng := rand.Float32()
			if rng > sng.GlobalDigestOverlap+sng.TraceDigestOverlap {
				// new digest
				newD := randomDigest(sng.TestName)
				digests[newD] = true
			} else if rng < sng.GlobalDigestOverlap {
				// global digest
				digests[globalDigests[rand.Intn(len(globalDigests))]] = true
			} else {
				// shared digest or new digest if testWideDigests are empty
				if len(testWideDigests) > 0 {
					digests[testWideDigests[rand.Intn(len(testWideDigests))]] = true
				} else {
					newD := randomDigest(sng.TestName)
					digests[newD] = true
				}
			}
		}
		unique := types.DigestSet{}
		unique.AddLists(testWideDigests, digests.Keys())
		testWideDigests = unique.Keys()
	}

	idxs := make([]int, sng.NumCommits)
	for i := range idxs {
		idxs[i] = i
	}

	// Fill out each trace accordingly
	for i := range rv {
		trace := rv[i].Trace
		digests := digestsPerTrace[i].Keys()
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

func r(min, max int) int {
	return rand.Intn(max-min) + min
}
func rf(min, max float32) float32 {
	return rand.Float32()*(max-min) + min
}

var globalDigests = []types.Digest{
	"00000000000000000000000000000000",
	"11111111111111111111111111111111",
	"22222222222222222222222222222222",
	"33333333333333333333333333333333",
	"44444444444444444444444444444444",
	"55555555555555555555555555555555",
	"66666666666666666666666666666666",
	"77777777777777777777777777777777",
	"88888888888888888888888888888888",
	"99999999999999999999999999999999",
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
