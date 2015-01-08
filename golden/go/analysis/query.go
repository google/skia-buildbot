package analysis

import (
	"sort"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/util"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

const (
	QUERY_COMMIT_START = "cs"
	QUERY_COMMIT_END   = "ce"
)

// ParamIndex maps parameter key/value pairs to unique trace ids.
type ParamIndex map[string]map[string][]int

type LabeledTileIndex struct {
	AllParams map[string][]string
	AllTraces []*LabeledTrace

	paramIndex ParamIndex
	traceMap   map[int]*LabeledTrace
	commits    []*ptypes.Commit
	commitsMap map[string]int
}

func NewLabeledTileIndex(labeledTile *LabeledTile) *LabeledTileIndex {
	paramIndex, traceMap, commitsMap, allParams, allTraces := buildIndex(labeledTile)

	return &LabeledTileIndex{
		AllParams:  allParams,
		AllTraces:  allTraces,
		paramIndex: paramIndex,
		traceMap:   traceMap,
		commits:    labeledTile.Commits,
		commitsMap: commitsMap,
	}
}

// query find all traces that match the given query which contains a
// set of parameters and specific values. It also returns the subset of 'query'
// that contained correct parameters and values and was used in the lookup.
func (i *LabeledTileIndex) query(query, effectiveQuery map[string][]string) ([]*LabeledTrace, int, int) {
	startCommit, endCommit := i.getCommitRange(query, effectiveQuery)
	traces := i.queryParams(query, effectiveQuery)
	return traces, startCommit, endCommit
}

// getCommitRange Returns the index of the first and last commit as identified
// by the query.
func (i *LabeledTileIndex) getCommitRange(query, effectiveQuery map[string][]string) (int, int) {
	// Look up a query string and return the default value if not found.
	getCommit := func(qStr string, defVal int) int {
		if val, ok := query[qStr]; ok {
			if len(val) != 1 {
				return defVal
			}
			if ret, ok := i.commitsMap[val[0]]; ok {
				effectiveQuery[qStr] = query[qStr]
				return ret
			}
		}
		return defVal
	}

	startCommit := getCommit(QUERY_COMMIT_START, 0)
	endCommit := getCommit(QUERY_COMMIT_END, len(i.commits)-1)

	if endCommit < startCommit {
		startCommit, endCommit = endCommit, startCommit
	}

	// Set endCommit up to be non-exclusive.
	return startCommit, endCommit
}

func (i *LabeledTileIndex) queryParams(query, effectiveQuery map[string][]string) []*LabeledTrace {
	resultSets := make([]map[int]bool, 0, len(query))

	var tempSet map[int]bool = nil
	minIdx, minLen := 0, 0
	for key, values := range query {
		if paramMap, ok := i.paramIndex[key]; ok {
			tempVals := make([]string, 0, len(values))
			tempSet = map[int]bool{}
			for _, v := range values {
				if indexList, ok := paramMap[v]; ok {
					for _, labelId := range indexList {
						tempSet[labelId] = true
					}
					tempVals = append(tempVals, v)
				}
			}

			// Only consider if at least on value in the query was valid.
			if len(tempVals) > 0 {
				effectiveQuery[key] = tempVals
			}

			// Record the minimum length if it's smaller or we are in the first
			// run of the loop.
			if (len(tempSet) < minLen) || (minLen == 0) {
				minIdx = len(resultSets)
				minLen = len(tempSet)
			}
			resultSets = append(resultSets, tempSet)
		}
	}

	// Check if we had any valid parameters. This also covers the case
	// when we have only commit range selectors in query.
	if len(resultSets) == 0 {
		return i.AllTraces
	}

	resultIds := util.IntersectIntSets(resultSets, minIdx)
	result := make([]*LabeledTrace, 0, len(resultIds))
	for _, id := range resultIds {
		if lt, ok := i.traceMap[id]; ok {
			result = append(result, lt)
		}
	}
	return result
}

// TODO(stephana): This needs to be folded into analysis.go and
// the different loops that iterate over the entire tile need to be
// consolidated into one loop that call the various functions to calculate
// counts, aggregates etc.
// Each group of functions should be in it's own source file (similar to
// triage.go) with analysis.go being the main file of that package.

// buildIndex takes the labeled tile and generates an index to look up the
// traces via parameter values. It returns the parameter index, a mapping
// of ids to traces and a map of all parameters and their values.
func buildIndex(labeledTile *LabeledTile) (ParamIndex, map[int]*LabeledTrace, map[string]int, map[string][]string, []*LabeledTrace) {
	glog.Info("Building parameter index.")

	// build the lookup index for commits
	commitsMap := make(map[string]int, len(labeledTile.Commits))
	for idx, commit := range labeledTile.Commits {
		commitsMap[commit.Hash] = idx
	}

	index := map[string]map[string][]int{}
	traceMap := map[int]*LabeledTrace{}
	allTraces := make([]*LabeledTrace, 0, len(labeledTile.Traces))

	for _, testTraces := range labeledTile.Traces {
		for _, oneTrace := range testTraces {
			traceMap[oneTrace.Id] = oneTrace
			for k, v := range oneTrace.Params {
				if _, ok := index[k]; !ok {
					index[k] = map[string][]int{}
				}
				if _, ok := index[k][v]; !ok {
					index[k][v] = []int{}
				}
				index[k][v] = append(index[k][v], oneTrace.Id)
			}
			allTraces = append(allTraces, oneTrace)
		}
	}

	allParams := make(map[string][]string, len(index))
	for param, values := range index {
		allParams[param] = make([]string, 0, len(values))
		for k := range values {
			allParams[param] = append(allParams[param], k)
		}
		sort.Strings(allParams[param])
	}

	glog.Info("Done building parameter index.")
	return index, traceMap, commitsMap, allParams, allTraces
}
