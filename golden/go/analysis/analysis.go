package analysis

import (
	"sort"
	"sync"
	"time"

	"github.com/golang/glog"
	metrics "github.com/rcrowley/go-metrics"

	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	"skia.googlesource.com/buildbot.git/perf/go/human"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

type PathToURLConverter func(string) string

// LabeledTrace stores a Trace with labels and digests. CommitIds, Digests and
// Labels are of the same length, identical indices refer to the same digest.
type LabeledTrace struct {
	Params    map[string]string
	CommitIds []int
	Digests   []string
	Labels    []types.Label
	Id        int
}

func NewLabeledTrace(params map[string]string, capacity int, traceId int) *LabeledTrace {
	return &LabeledTrace{
		Params:    params,
		CommitIds: make([]int, 0, capacity),
		Digests:   make([]string, 0, capacity),
		Labels:    make([]types.Label, 0, capacity),
		Id:        traceId,
	}
}

// addLabledDigests adds the given tripples of commitIds, digests and labels to this LabeledTrace.
func (lt *LabeledTrace) addLabeledDigests(commitIds []int, digests []string, labels []types.Label) {
	lt.CommitIds = append(lt.CommitIds, commitIds...)
	lt.Digests = append(lt.Digests, digests...)
	lt.Labels = append(lt.Labels, labels...)
}

// LabeledTile aggregates the traces of a tile and provides a slice of commits
// that the commitIds in LabeledTrace refer to.
// LabeledTile and LabeledTrace store the cannonical information
// extracted from the unterlying tile store. The (redundant) output data is
// derived from these.
type LabeledTile struct {
	Commits []*ptypes.Commit

	// Traces are indexed by the primary key (test name). This is somewhat
	// redundant, but this also output format.
	Traces map[string][]*LabeledTrace

	// Set of all parameters and their values.
	allParams map[string][]string

	// Keeps track of unique ids for traces within this tile.
	traceIdCounter int
}

func NewLabeledTile() *LabeledTile {
	return &LabeledTile{
		Commits:        []*ptypes.Commit{},
		Traces:         map[string][]*LabeledTrace{},
		traceIdCounter: 0,
	}
}

// getLabeledTrace is a utility function that returns the testName and a labeled
// trace for the given trace (read from a TileStore). If the LabeledTrace does
// not exist it will be added.
func (t *LabeledTile) getLabeledTrace(trace ptypes.Trace) (string, *LabeledTrace) {
	params := trace.Params()
	pKey := params[types.PRIMARY_KEY_FIELD]
	if _, ok := t.Traces[pKey]; !ok {
		// Add the primary key with a single labled trace.
		t.Traces[pKey] = []*LabeledTrace{}
	}

	// Search through the traces associated witht this test.
	for _, v := range t.Traces[pKey] {
		if util.MapsEqual(v.Params, params) {
			return pKey, v
		}
	}

	// If we cannot find the trace in our set of tests we are adding a new
	// labeled trace.
	newLT := NewLabeledTrace(params, trace.Len(), t.traceIdCounter)
	t.traceIdCounter++
	t.Traces[pKey] = append(t.Traces[pKey], newLT)
	return pKey, newLT
}

// LabelCounts is an output type to hold counts for classification labels.
type LabelCounts struct {
	Unt []int `json:"unt"` // Untriaged
	Pos []int `json:"pos"` // Positive
	Neg []int `json:"neg"` // Negative
}

func newLabelCounts(length int) *LabelCounts {
	return &LabelCounts{
		Unt: make([]int, length),
		Pos: make([]int, length),
		Neg: make([]int, length),
	}
}

// GUITileCounts is an output type for the aggregated label counts.
type GUITileCounts struct {
	Commits    []*ptypes.Commit        `json:"commits"`
	Ticks      []interface{}           `json:"ticks"`
	Aggregated *LabelCounts            `json:"aggregated"`
	Counts     map[string]*LabelCounts `json:"counts"`
	AllParams  map[string][]string     `json:"allParams"`
	Query      map[string][]string     `json:"query"`
}

// Analyzer continuously manages tasks like polling for new traces
// on disk and generating diffs between images. It is the primary interface
// to be called by the HTTP frontend.
type Analyzer struct {
	expStore  expstorage.ExpectationsStore
	diffStore diff.DiffStore
	tileStore ptypes.TileStore

	// Canonical data structure to hold our information about commits, digests
	// and labels.
	currentTile *LabeledTile

	// Index to query the current tile.
	currentIndex *LabeledTileIndex

	// Output data structures that are derived from currentTile.
	currentTileCounts  *GUITileCounts
	currentTestDetails *GUITestDetails
	currentStatus      *GUIStatus

	// Maps from trace ids to the actual instances of LabeledTrace.
	traceMap map[int]*LabeledTrace

	// converter supplied by the client of the type to convert a path to a URL
	pathToURLConverter PathToURLConverter

	// Lock to protect the expectations and current* variables.
	mutex sync.RWMutex

	// Counts the number of times the main event loop has executed.
	// This is for testing only.
	loopCounter int
}

func NewAnalyzer(expStore expstorage.ExpectationsStore, tileStore ptypes.TileStore, diffStore diff.DiffStore, puConverter PathToURLConverter, timeBetweenPolls time.Duration) *Analyzer {
	result := &Analyzer{
		expStore:           expStore,
		diffStore:          diffStore,
		tileStore:          tileStore,
		pathToURLConverter: puConverter,

		currentTile: NewLabeledTile(),
	}

	go result.loop(timeBetweenPolls)
	return result
}

// GetTileCounts returns an entire Tile which is a collection of 'traces' over
// a series of commits. Each trace contains the digests and their labels
// based on our knowledge about digests (expectations).
func (a *Analyzer) GetTileCounts(query map[string][]string) (*GUITileCounts, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if len(query) > 0 {
		tile, effectiveQuery := a.getSubTile(query)
		if len(effectiveQuery) > 0 {
			ret := a.getOutputCounts(tile)
			ret.Query = effectiveQuery
			return ret, nil
		}
	}

	return a.currentTileCounts, nil
}

// ListTestDetails returns a list of triage details based on the supplied
// query. It's complementary to GetTestDetails which returns a single test
// detail.
// TODO(stephana): This should provide pagination since the list is potentially
// very long. If we don't add pagination, this should be merged with
// GetTestDetail.
func (a *Analyzer) ListTestDetails(query map[string][]string) (*GUITestDetails, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if len(query) == 0 {
		return a.currentTestDetails, nil
	}

	effectiveQuery := make(map[string][]string, len(query))
	foundUntriaged := a.getUntriagedTestDetails(query, effectiveQuery, true)
	tests := make([]*GUITestDetail, 0, len(foundUntriaged))

	for testName, untriaged := range foundUntriaged {
		testDetail := a.currentTestDetails.lookup(testName)
		tests = append(tests, &GUITestDetail{
			Name:      testName,
			Untriaged: untriaged,
			Positive:  testDetail.Positive,
			Negative:  testDetail.Negative,
		})
	}

	// Sort the test details.
	sort.Sort(GUITestDetailSortable(tests))

	return &GUITestDetails{
		Commits:   a.currentTestDetails.Commits,
		AllParams: a.currentTestDetails.AllParams,
		Query:     effectiveQuery,
		Tests:     tests,
	}, nil
}

// GetTestDetails returns the untriaged, positive and negative digests for a
// specific test with the necessary information (diff metrics, image urls) to
// assign a label to the untriaged digests.
// If query is not empty then we will return traces that match the query.
// If the query is empty and testName is not empty we will return the
// traces of the corresponding test.If both query and testName are empty
// we will return all traces.
// TODO (stephana): If the result is too big we should add pagination.
func (a *Analyzer) GetTestDetails(testName string, query map[string][]string) (*GUITestDetails, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var effectiveQuery map[string][]string
	testDetail := a.currentTestDetails.lookup(testName)
	untriaged := testDetail.Untriaged
	if len(query) > 0 {
		effectiveQuery = map[string][]string{}

		// Filter by only this test.
		query[types.PRIMARY_KEY_FIELD] = []string{testName}
		foundUntriaged := a.getUntriagedTestDetails(query, effectiveQuery, false)
		delete(effectiveQuery, types.PRIMARY_KEY_FIELD)

		// Only consider the result if some query parameters were valid.
		if len(effectiveQuery) > 0 {
			if temp, ok := foundUntriaged[testName]; ok {
				untriaged = temp
			} else {
				untriaged = map[string]*GUIUntriagedDigest{}
			}
		}
	}

	return &GUITestDetails{
		Commits:   a.currentTestDetails.Commits,
		AllParams: a.currentTestDetails.AllParams,
		Query:     effectiveQuery,
		Tests: []*GUITestDetail{
			&GUITestDetail{
				Name:      testName,
				Untriaged: untriaged,
				Positive:  testDetail.Positive,
				Negative:  testDetail.Negative,
			},
		},
	}, nil
}

// SetDigestLabels sets the labels for the given digest and records the user
// that made the classification.
func (a *Analyzer) SetDigestLabels(labeledTestDigests map[string]types.TestClassification, userId string) (*GUITestDetails, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	expectations, err := a.expStore.Get(true)
	if err != nil {
		return nil, err
	}
	expectations.AddDigests(labeledTestDigests)
	if err = a.expStore.Put(expectations, userId); err != nil {
		return nil, err
	}

	// Let's update our knowledge of the labels.
	a.updateDerivedOutputs(labeledTestDigests, expectations)

	result := make([]*GUITestDetail, 0, len(labeledTestDigests))
	for testName := range labeledTestDigests {
		result = append(result, a.currentTestDetails.lookup(testName))
	}

	return &GUITestDetails{
		Commits:   a.currentTestDetails.Commits,
		AllParams: a.currentTestDetails.AllParams,
		Tests:     result,
	}, nil
}

func (a *Analyzer) GetStatus() *GUIStatus {
	return a.currentStatus
}

// loop is the main event loop.
func (a *Analyzer) loop(timeBetweenPolls time.Duration) {
	// The number of times we've successfully loaded and processed a tile.
	runsCounter := metrics.NewRegisteredCounter("analysis.runs", metrics.DefaultRegistry)

	// The number of times an error has ocurred when trying to load a tile.
	errorTileLoadingCounter := metrics.NewRegisteredCounter("analysis.errors", metrics.DefaultRegistry)

	processOneTile := func() {
		glog.Info("Reading tiles ... ")

		// Load the tile and process it.
		tile, err := a.tileStore.Get(0, -1)
		glog.Info("Done reading tiles.")

		if err != nil {
			glog.Errorf("Error reading tile store: %s\n", err)
			errorTileLoadingCounter.Inc(1)
		} else {
			// Protect the tile and expectations with the write lock.
			a.mutex.Lock()
			defer a.mutex.Unlock()

			// Retrieve the current expectations.
			expectations, err := a.expStore.Get(false)
			if err != nil {
				glog.Errorf("Error retrieving expectations: %s", err)
				return
			}

			newLabeledTile := a.processTile(tile)
			a.setDerivedOutputs(newLabeledTile, expectations)
		}
		glog.Info("Done processing tiles.")
		runsCounter.Inc(1)
		a.loopCounter++
	}

	// process a tile immediately and then at fixed points in time.
	processOneTile()
	for _ = range time.Tick(timeBetweenPolls) {
		processOneTile()
	}
}

// processTile processes the last two tiles and updates the cannonical and
// output data structures.
func (a *Analyzer) processTile(tile *ptypes.Tile) *LabeledTile {
	glog.Info("Processing tile into LabeledTile ...")
	result := NewLabeledTile()

	tileLen := tile.LastCommitIndex() + 1
	result.Commits = tile.Commits[:tileLen]

	// Note: We are assumming that the number and order of traces will change
	// over time.
	for _, v := range tile.Traces {
		tempCommitIds := make([]int, 0, tileLen)
		tempLabels := make([]types.Label, 0, tileLen)
		tempDigests := make([]string, 0, tileLen)
		gTrace := v.(*ptypes.GoldenTrace)

		// Iterate over the digests in this trace.
		for i, v := range gTrace.Values[:tileLen] {
			if gTrace.Values[i] != ptypes.MISSING_DIGEST {
				tempCommitIds = append(tempCommitIds, i)
				tempDigests = append(tempDigests, v)
				tempLabels = append(tempLabels, types.UNTRIAGED)
			}
		}

		// Label the digests and add them to the labeled traces.
		_, targetLabeledTrace := result.getLabeledTrace(v)
		targetLabeledTrace.addLabeledDigests(tempCommitIds, tempDigests, tempLabels)
	}

	glog.Info("Done processing tile into LabeledTile.")
	return result
}

// setDerivedOutputs derives the output data from the given tile and
// updates the outputs and tile in the analyzer.
func (a *Analyzer) setDerivedOutputs(labeledTile *LabeledTile, expectations *expstorage.Expectations) {
	// Assign all the labels.
	for testName, traces := range labeledTile.Traces {
		for _, trace := range traces {
			a.labelDigests(testName, trace.Digests, trace.Labels, expectations)
		}
	}

	// Generate the lookup index for the tile and get all parameters.
	a.currentIndex = NewLabeledTileIndex(labeledTile)
	labeledTile.allParams = a.currentIndex.AllParams

	// calculate all the output data.
	a.currentTile = labeledTile
	a.currentTileCounts = a.getOutputCounts(labeledTile)
	a.currentTestDetails = a.getTestDetails(labeledTile)
	a.currentStatus = calcStatus(labeledTile)
}

// updateLabels iterates over the traces in of the tiles that have changed and
// labels them according to our current expecatations.

// updateDerivedOutputs
func (a *Analyzer) updateDerivedOutputs(labeledTestDigests map[string]types.TestClassification, expectations *expstorage.Expectations) {
	// Update the labels of the traces that have changed.
	for testName := range labeledTestDigests {
		if traces, ok := a.currentTile.Traces[testName]; ok {
			for _, trace := range traces {
				// Note: This is potentially slower than using labels in
				// labeledTestDigests directly, but it keeps the code simpler.
				a.labelDigests(testName, trace.Digests, trace.Labels, expectations)
			}
		}
	}

	// Update all the output data structures.
	// TODO(stephana): Evaluate whether the counts are really useful or if they can be removed.
	// If we need them uncomment the following line and implement the corresponding function.
	//a.updateOutputCounts(labeledTestDigests)

	// Update the tests that have changed and the status.
	a.updateTestDetails(labeledTestDigests)
	a.currentStatus = calcStatus(a.currentTile)
}

// labelDigest assignes a label to the given digests based on the expectations.
// Its assumes that targetLabels are pre-initialized, usualy with UNTRIAGED,
// because it will not change the label if the given test and digest cannot be
// found.
func (a *Analyzer) labelDigests(testName string, digests []string, targetLabels []types.Label, expectations *expstorage.Expectations) {
	for idx, digest := range digests {
		if test, ok := expectations.Tests[testName]; ok {
			if foundLabel, ok := test[digest]; ok {
				targetLabels[idx] = foundLabel
			}
		}
	}
}

// getUntriagedDigests returns the untriaged digests of a specific test that
// match the given query. In addition to the digests it returns the query
// that was used to retrieve them.
func (a *Analyzer) getUntriagedTestDetails(query, effectiveQuery map[string][]string, includeAllTests bool) map[string]map[string]*GUIUntriagedDigest {
	traces, startCommitId, endCommitId := a.currentIndex.query(query, effectiveQuery)
	endCommitId++

	if len(effectiveQuery) == 0 {
		return nil
	}

	ret := make(map[string]map[string]*GUIUntriagedDigest, len(a.currentTestDetails.Tests))
	for _, trace := range traces {
		testName := trace.Params[types.PRIMARY_KEY_FIELD]
		current := a.currentTestDetails.lookup(testName).Untriaged

		startIdx := sort.SearchInts(trace.CommitIds, startCommitId)
		endIdx := sort.SearchInts(trace.CommitIds, endCommitId)
		if (endIdx < len(trace.CommitIds)) && (trace.CommitIds[endIdx] == endCommitId) {
			endIdx++
		}

		for idx := startIdx; idx < endIdx; idx++ {
			if trace.Labels[idx] == types.UNTRIAGED {
				if found, ok := ret[testName]; !ok || found == nil {
					ret[testName] = make(map[string]*GUIUntriagedDigest, len(current))
				}
				ret[testName][trace.Digests[idx]] = current[trace.Digests[idx]]
			}
		}

		if includeAllTests {
			if _, ok := ret[testName]; !ok {
				ret[testName] = nil
			}
		}
	}
	return ret
}

// getSubTile queries the index and returns a LabeledTile that contains the
// set of found traces. It also returns the subset of 'query' that contained
// valid parameters and values.
// If the returned query is empty the first return value is set to Nil,
// because now valid filter parameters were found in the query.
func (a *Analyzer) getSubTile(query map[string][]string) (*LabeledTile, map[string][]string) {
	// TODO(stephana): Use the commitStart and commitEnd return values
	// if we really need this method. GetTileCounts and getSubTile might be
	// removed.
	effectiveQuery := make(map[string][]string, len(query))
	traces, _, _ := a.currentIndex.query(query, effectiveQuery)
	if len(effectiveQuery) == 0 {
		return nil, effectiveQuery
	}

	result := NewLabeledTile()
	result.Commits = a.currentTile.Commits
	result.allParams = a.currentTile.allParams

	result.Traces = map[string][]*LabeledTrace{}
	for _, t := range traces {
		testName := t.Params[types.PRIMARY_KEY_FIELD]
		if _, ok := result.Traces[testName]; !ok {
			result.Traces[testName] = []*LabeledTrace{}
		}
		result.Traces[testName] = append(result.Traces[testName], t)
	}

	return result, effectiveQuery
}

// getOutputCounts derives the output counts from the given labeled tile.
func (a *Analyzer) getOutputCounts(labeledTile *LabeledTile) *GUITileCounts {
	glog.Info("Starting to process output counts.")
	// Stores the aggregated counts of a tile for each test.
	tileCountsMap := make(map[string]*LabelCounts, len(labeledTile.Traces))

	// Overall aggregated counts over all tests.
	overallAggregates := newLabelCounts(len(labeledTile.Commits))

	updateCounts(labeledTile, tileCountsMap, overallAggregates)

	// TODO (stephana): Factor out human.FlotTickMarks and move it from
	// perf to the shared go library.
	// Generate the tickmarks for the commits.
	ts := make([]int64, 0, len(labeledTile.Commits))
	for _, c := range labeledTile.Commits {
		if c.CommitTime != 0 {
			ts = append(ts, c.CommitTime)
		}
	}

	tileCounts := &GUITileCounts{
		Commits:    labeledTile.Commits,
		Ticks:      human.FlotTickMarks(ts),
		Aggregated: overallAggregates,
		Counts:     tileCountsMap,
		AllParams:  labeledTile.allParams,
	}

	glog.Info("Done processing output counts.")

	return tileCounts
}

func updateCounts(labeledTile *LabeledTile, tileCountsMap map[string]*LabelCounts, overallAggregates *LabelCounts) {
	for testName, testTraces := range labeledTile.Traces {
		acc := newLabelCounts(len(labeledTile.Commits))

		for _, oneTrace := range testTraces {
			for i, ci := range oneTrace.CommitIds {
				switch oneTrace.Labels[i] {
				case types.UNTRIAGED:
					acc.Unt[ci]++
				case types.POSITIVE:
					acc.Pos[ci]++
				case types.NEGATIVE:
					acc.Neg[ci]++
				}
			}
		}

		tileCountsMap[testName] = acc

		// Add the aggregates fro this test to the overall aggregates.
		for idx, u := range acc.Unt {
			overallAggregates.Unt[idx] += u
			overallAggregates.Pos[idx] += acc.Pos[idx]
			overallAggregates.Neg[idx] += acc.Neg[idx]
		}
	}
}
