package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/search"
)

// TODO(stephana): once the byBlameHandler is removed, refactor this to
// remove the redundant types ByBlameEntry and ByBlame.

// jsonByBlameHandler returns a json object with the digests to be triaged grouped by blamelist.
func jsonByBlameHandler(w http.ResponseWriter, r *http.Request) {
	tile, sum, err := allUntriagedSummaries()
	commits := tile.Commits
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to load summaries.")
		return
	}

	// This is a very simple grouping of digests, for every digest we look up the
	// blame list for that digest and then use the concatenated git hashes as a
	// group id. All of the digests are then grouped by their group id.

	// Collects a ByBlame for each untriaged digest, keyed by group id.
	grouped := map[string][]*ByBlame{}

	// The Commit info for each group id.
	commitinfo := map[string][]*tiling.Commit{}
	// map [groupid] [test] TestRollup
	rollups := map[string]map[string]*TestRollup{}

	for test, s := range sum {
		for _, d := range s.UntHashes {
			dist := blamer.GetBlame(test, d, commits)
			groupid := strings.Join(lookUpCommits(dist.Freq, commits), ":")
			// Only fill in commitinfo for each groupid only once.
			if _, ok := commitinfo[groupid]; !ok {
				ci := []*tiling.Commit{}
				for _, index := range dist.Freq {
					ci = append(ci, commits[index])
				}
				sort.Sort(CommitSlice(ci))
				commitinfo[groupid] = ci
			}
			// Construct a ByBlame and add it to grouped.
			value := &ByBlame{
				Test:          test,
				Digest:        d,
				Blame:         dist,
				CommitIndices: dist.Freq,
			}
			if _, ok := grouped[groupid]; !ok {
				grouped[groupid] = []*ByBlame{value}
			} else {
				grouped[groupid] = append(grouped[groupid], value)
			}
			if _, ok := rollups[groupid]; !ok {
				rollups[groupid] = map[string]*TestRollup{}
			}
			// Calculate the rollups.
			if _, ok := rollups[groupid][test]; !ok {
				rollups[groupid][test] = &TestRollup{
					Test:         test,
					Num:          0,
					SampleDigest: d,
				}
			}
			rollups[groupid][test].Num += 1
		}
	}

	// Assemble the response.
	blameEntries := make([]*ByBlameEntry, 0, len(grouped))
	for groupid, byBlames := range grouped {
		rollup := rollups[groupid]
		nTests := len(rollup)
		var affectedTests []*TestRollup = nil

		// Only include the affected tests if there are no more than 10 of them.
		if nTests <= 10 {
			affectedTests = make([]*TestRollup, 0, nTests)
			for _, testInfo := range rollup {
				affectedTests = append(affectedTests, testInfo)
			}
		}

		blameEntries = append(blameEntries, &ByBlameEntry{
			GroupID:       groupid,
			NDigests:      len(byBlames),
			NTests:        nTests,
			AffectedTests: affectedTests,
			Commits:       commitinfo[groupid],
		})
	}
	sort.Sort(ByBlameEntrySlice(blameEntries))

	// Wrap the result in an object because we don't want to return
	// a JSON array.
	sendJsonResponse(w, map[string]interface{}{"data": blameEntries})
}

// ByBlameEntry is a helper structure that is serialized to
// JSON and sent to the front-end.
type ByBlameEntry struct {
	GroupID       string           `json:"groupID"`
	NDigests      int              `json:"nDigests"`
	NTests        int              `json:"nTests"`
	AffectedTests []*TestRollup    `json:"affectedTests"`
	Commits       []*tiling.Commit `json:"commits"`
}

type ByBlameEntrySlice []*ByBlameEntry

func (b ByBlameEntrySlice) Len() int           { return len(b) }
func (b ByBlameEntrySlice) Less(i, j int) bool { return b[i].GroupID < b[j].GroupID }
func (b ByBlameEntrySlice) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }

// jsonSearchHandler is the endpoint for all searches.
func jsonSearchHandler(w http.ResponseWriter, r *http.Request) {
	searchResponse, err := search.Search(queryFromRequest(r), storages, tallies, blamer, paramsetSum)
	if err != nil {
		httputils.ReportError(w, r, err, "Search for digests failed.")
		return
	}
	sendJsonResponse(w, &SearchResult{
		Digests: searchResponse.Digests,
		Commits: searchResponse.Commits,
	})
}

// SearchResult encapsulates the results of a search request.
type SearchResult struct {
	Digests    []*search.Digest `json:"digests"`
	Commits    []*tiling.Commit `json:"commits"`
	NumMatches int
}

// TODO(stephana): Remove polyDiffJSONDigestHandler and polyDetailsHandler once all
// detail and diff request go through jsonDetailsHandler and jsonDiffHandler.

// jsonDetailsHandler returns the details about a single digest.
func jsonDetailsHandler(w http.ResponseWriter, r *http.Request) {
	// Extract: test, digest.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	digest := r.Form.Get("digest")
	if test == "" || digest == "" {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are missing: %q %q", test, digest), "Missing query parameters.")
		return
	}

	ret, err := search.GetDigestDetails(test, digest, storages, paramsetSum, tallies)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to get digest details.")
		return
	}
	sendJsonResponse(w, ret)
}

// jsonDiffHandler returns difference between two digests.
func jsonDiffHandler(w http.ResponseWriter, r *http.Request) {
	// Extract: test, left, right where left and right are digests.
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form values")
		return
	}
	test := r.Form.Get("test")
	left := r.Form.Get("left")
	right := r.Form.Get("right")
	if test == "" || left == "" || right == "" {
		httputils.ReportError(w, r, fmt.Errorf("Some query parameters are missing: %q %q %q", test, left, right), "Missing query parameters.")
		return
	}

	ret, err := search.CompareDigests(test, left, right, storages, paramsetSum)
	if err != nil {
		httputils.ReportError(w, r, err, "Unable to compare digests")
		return
	}

	sendJsonResponse(w, ret)
}
