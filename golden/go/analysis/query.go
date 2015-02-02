package analysis

import (
	"sort"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

const (
	QUERY_COMMIT_START = "cs"
	QUERY_COMMIT_END   = "ce"
	QUERY_HEAD         = "head"
)

var (
	QUERY_HEAD_FALSE = []string{"0"}
	QUERY_HEAD_TRUE  = []string{"1"}
)

// ParamIndex maps parameter key/value pairs to unique trace ids.
type ParamIndex map[string]map[string][]int

type LabeledTileIndex struct {
	AllTraces []*LabeledTrace

	allParams         map[string][]string
	corpora           []string
	paramsByCorpus    map[string]map[string][]string
	allTestNames      []string
	testNamesByCorpus map[string][]string
	traceIndex        ParamIndex
	traceMap          map[int]*LabeledTrace
	commits           []*ptypes.Commit
	commitsMap        map[string]int
}

func NewLabeledTileIndex(labeledTile *LabeledTile) *LabeledTileIndex {
	ret := &LabeledTileIndex{}
	ret.buildIndex(labeledTile)
	return ret
}

// query find all traces that match the given query which contains a
// set of parameters and specific values.
// It returns the array of traces that match the query, the indices of the
// first and last commit to consider and whether the head flag was set to
// true. In the latter case any commit range query will be ignored.
// All query values used in producing the result will be added to
// effectiveQuery.
func (i *LabeledTileIndex) query(query, effectiveQuery map[string][]string) ([]*LabeledTrace, int, int, bool) {
	// Figure out whether we are just interested in HEAD or everything.
	headVal, headValueSet := query[QUERY_HEAD]
	if !headValueSet {
		headVal = QUERY_HEAD_TRUE
	}

	showHead := (len(headVal) == 0) || (headVal[0] != QUERY_HEAD_FALSE[0])
	if showHead {
		delete(query, QUERY_COMMIT_START)
		delete(query, QUERY_COMMIT_END)
	}
	effectiveQuery[QUERY_HEAD] = headVal

	startCommit, endCommit := i.getCommitRange(query, effectiveQuery)
	traces := i.queryTraces(query, effectiveQuery)
	return traces, startCommit, endCommit, showHead
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

func (i *LabeledTileIndex) queryTraces(query, effectiveQuery map[string][]string) []*LabeledTrace {
	resultSets := make([]map[int]bool, 0, len(query))

	var tempSet map[int]bool = nil
	minIdx, minLen := 0, 0
	for key, values := range query {
		if paramMap, ok := i.traceIndex[key]; ok {
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
	for id := range resultIds {
		if lt, ok := i.traceMap[id]; ok {
			result = append(result, lt)
		}
	}
	return result
}

// getAllParams returns all parameter relevant for the given query.
// It is corpus aware, if the corpus field was queried it will return
// only the parameters relevant to the given corpus.
func (li *LabeledTileIndex) getAllParams(query map[string][]string) map[string][]string {
	if ret, ok := li.paramsByCorpus[getCorpus(query)]; ok {
		return ret
	}
	return li.allParams
}

// getTestNames returns the list of test names for the given query.
// If the corpus is selected in the query it will only return the test names
// relevant for that corpus.
func (li *LabeledTileIndex) getTestNames(query map[string][]string) []string {
	if ret := li.testNamesByCorpus[getCorpus(query)]; ret != nil {
		return ret
	}
	return li.allTestNames
}

// Convenience function that returns the corpus name if present in query or "".
func getCorpus(query map[string][]string) string {
	if c := query[types.CORPUS_FIELD]; len(c) == 1 {
		return c[0]
	}
	return ""
}

// TODO(stephana): This needs to be folded into analysis.go and
// the different loops that iterate over the entire tile need to be
// consolidated into one loop that call the various functions to calculate
// counts, aggregates etc.
// Each group of functions should be in it's own source file (similar to
// triage.go) with analysis.go being the main file of that package.

// buildIndex takes the labeled tile and generates an index to look up the
// traces via parameters and other often requested data.
func (li *LabeledTileIndex) buildIndex(labeledTile *LabeledTile) {
	glog.Info("Building LabeledTileIndex.")

	// build the lookup index for commits
	commitsMap := make(map[string]int, len(labeledTile.Commits))
	for idx, commit := range labeledTile.Commits {
		commitsMap[commit.Hash] = idx
	}

	traceIndex := map[string]map[string][]int{}
	traceMap := map[int]*LabeledTrace{}
	allTraces := make([]*LabeledTrace, 0, len(labeledTile.Traces))
	paramsByCorpusMap := map[string]map[string]map[string]bool{}
	allTestNames := make([]string, 0, len(labeledTile.Traces))
	testNamesByCorpusMap := map[string]map[string]bool{}

	var corpus string
	var pbc map[string]map[string]bool
	var ok bool

	for testName, testTraces := range labeledTile.Traces {
		for _, oneTrace := range testTraces {
			traceMap[oneTrace.Id] = oneTrace

			// Keep track of the traces by corpus.
			corpus = oneTrace.Params[types.CORPUS_FIELD]
			if pbc, ok = paramsByCorpusMap[corpus]; !ok {
				pbc = map[string]map[string]bool{}
				paramsByCorpusMap[corpus] = pbc
			}

			for k, v := range oneTrace.Params {
				if _, ok := traceIndex[k]; !ok {
					traceIndex[k] = map[string][]int{}
				}
				if _, ok := traceIndex[k][v]; !ok {
					traceIndex[k][v] = []int{}
				}
				traceIndex[k][v] = append(traceIndex[k][v], oneTrace.Id)
				addToMap(pbc, k, v)
			}
			allTraces = append(allTraces, oneTrace)
			addToMap(testNamesByCorpusMap, corpus, testName)
		}
		allTestNames = append(allTestNames, testName)
	}

	allParams := make(map[string][]string, len(traceIndex))
	for param, values := range traceIndex {
		allParams[param] = make([]string, 0, len(values))
		for k := range values {
			allParams[param] = append(allParams[param], k)
		}
		sort.Strings(allParams[param])
	}

	corpora := make([]string, 0, len(paramsByCorpusMap))
	paramsByCorpus := make(map[string]map[string][]string, len(paramsByCorpusMap))
	testNamesByCorpus := make(map[string][]string, len(testNamesByCorpusMap))
	for corpus, corpusParams := range paramsByCorpusMap {
		paramsByCorpus[corpus] = make(map[string][]string, len(corpusParams))
		for param, vals := range corpusParams {
			paramsByCorpus[corpus][param] = util.KeysOfStringSet(vals)
			sort.Strings(paramsByCorpus[corpus][param])
		}

		corpora = append(corpora, corpus)
		testNamesByCorpus[corpus] = util.KeysOfStringSet(testNamesByCorpusMap[corpus])
	}

	glog.Info("Done building LabeledTileIndex.")

	li.allParams = allParams
	li.corpora = corpora
	li.paramsByCorpus = paramsByCorpus
	li.allTestNames = allTestNames
	li.testNamesByCorpus = testNamesByCorpus
	li.AllTraces = allTraces
	li.traceIndex = traceIndex
	li.traceMap = traceMap
	li.commits = labeledTile.Commits
	li.commitsMap = commitsMap
}

// Adds an new entry to the given map. It is assumed that key1 already exists.
func addToMap(current map[string]map[string]bool, key1, key2 string) {
	if _, ok := current[key1]; !ok {
		current[key1] = map[string]bool{key2: true}
	} else {
		current[key1][key2] = true
	}
}
