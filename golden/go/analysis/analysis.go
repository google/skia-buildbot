package analysis

import (
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/rcrowley/go-metrics"

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

// GUITestCounts is an output type for a single test that contains the
// aggregated counts over all traces and also the individual traces
// and their labels.
type GUITestCounts struct {
	Commits    []*ptypes.Commit   `json:"commits"`
	Aggregated *LabelCounts       `json:"aggregated"`
	Traces     []*GUILabeledTrace `json:"traces"`
}

// GUILabeledTrace is an output type for the labels of a trace.
type GUILabeledTrace struct {
	Params map[string]string `json:"params"`

	// List of commitId and Label pairs.
	Labels []IdLabel `json:"labels"`
}

// IdLabel stores the commitId and the label for one entry in a trace.
type IdLabel struct {
	Id    int `json:"id"`
	Label int `json:"label"`
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

	// Output data structures that are derived from currentTile.
	currentTileCounts  *GUITileCounts
	currentTestCounts  map[string]*GUITestCounts
	currentTestDetails *GUITestDetails

	// Index to query the current tile.
	index ParamIndex

	// Maps from trace ids to the actual instances of LabeledTrace.
	traceMap map[int]*LabeledTrace

	// converter supplied by the client of the type to convert a path to a URL
	pathToURLConverter PathToURLConverter

	// Lock to protect the expectations and current* variables.
	mutex sync.Mutex

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
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if len(query) > 0 {
		tile, effectiveQuery := a.getSubTile(query)
		if len(effectiveQuery) > 0 {
			ret, _ := a.getOutputCounts(tile)
			ret.Query = effectiveQuery
			return ret, nil
		}
	}

	return a.currentTileCounts, nil
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
	a.mutex.Lock()
	defer a.mutex.Unlock()

	if (testName == "") && (len(query) == 0) {
		return a.currentTestDetails, nil
	}

	if len(query) > 0 {
		tile, effectiveQuery := a.getSubTile(query)
		if len(effectiveQuery) > 0 {
			ret := a.getTestDetails(tile)
			ret.Query = effectiveQuery
			return ret, nil
		}
	}

	return &GUITestDetails{
		AllParams: a.currentTestDetails.AllParams,
		Tests:     map[string]*GUITestDetail{testName: a.currentTestDetails.Tests[testName]},
	}, nil
}

// GetTestCounts returns the classification counts for a specific tests.
func (a *Analyzer) GetTestCounts(testName string) (*GUITestCounts, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// TODO (stephana): This should return any error that occurs during reading
	// of the tiles. We would rather get an error on the front-end than
	// look at outdated data.
	return a.currentTestCounts[testName], nil
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
	a.partialUpdate(labeledTestDigests)
	a.setDerivedOutputs(a.currentTile, false)

	result := map[string]*GUITestDetail{}
	for testName := range labeledTestDigests {
		result[testName] = a.currentTestDetails.Tests[testName]
	}

	return &GUITestDetails{
		AllParams: a.currentTestDetails.AllParams,
		Tests:     result,
	}, nil
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
			glog.Errorf("Error reading tile store: %s\n", err.Error())
			errorTileLoadingCounter.Inc(1)
		} else {
			newLabeledTile := a.processTile(tile)
			a.setDerivedOutputs(newLabeledTile, true)
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
		testName, targetLabeledTrace := result.getLabeledTrace(v)
		if err := a.labelDigests(testName, tempDigests, tempLabels); err != nil {
			glog.Errorf("Error labeling digests: %s\n", err.Error())
			continue
		}
		targetLabeledTrace.addLabeledDigests(tempCommitIds, tempDigests, tempLabels)
	}

	glog.Info("Done processing tile into LabeledTile.")
	return result
}

// setDerivedOutputs derives the output data from the given tile and
// updates the outputs and tile in the analyzer. If needsLocking is true
// it will acquire the lock otherwise it assumes the calling function owns it.
func (a *Analyzer) setDerivedOutputs(labeledTile *LabeledTile, needsLocking bool) {
	// Generate the lookup index for the tile and get all parameters.
	var allParams map[string][]string
	a.index, a.traceMap, allParams = getQueryIndex(labeledTile)
	labeledTile.allParams = allParams

	// calculate all the output data.
	newTileCounts, newTestCounts := a.getOutputCounts(labeledTile)
	newTestDetails := a.getTestDetails(labeledTile)

	// acquire the lock if necessary
	if needsLocking {
		a.mutex.Lock()
		defer a.mutex.Unlock()
	}

	// update the analyzer's data structures
	a.currentTile = labeledTile
	a.currentTileCounts = newTileCounts
	a.currentTestCounts = newTestCounts
	a.currentTestDetails = newTestDetails
}

// partialUpdate iterates over the traces in of the tiles that have changed and
// labels them according to our current expecatations.
func (a *Analyzer) partialUpdate(labeledTestDigests map[string]types.TestClassification) {
	for testName := range labeledTestDigests {
		if traces, ok := a.currentTile.Traces[testName]; ok {
			for _, trace := range traces {
				// Note: This is potentially slower than using labels in
				// labeledTestDigests directly, but it keeps the code simpler.
				a.labelDigests(testName, trace.Digests, trace.Labels)
			}
		}
	}
}

// labelDigest assignes a label to the given digests based on the expectations.
// Its assumes that targetLabels are pre-initialized, usualy with UNTRIAGED,
// because it will not change the label if the given test and digest cannot be
// found.
func (a *Analyzer) labelDigests(testName string, digests []string, targetLabels []types.Label) error {
	expectations, err := a.expStore.Get(false)
	if err != nil {
		return err
	}

	for idx, digest := range digests {
		if test, ok := expectations.Tests[testName]; ok {
			if foundLabel, ok := test[digest]; ok {
				targetLabels[idx] = foundLabel
			}
		}
	}

	return nil
}

// getSubTile queries the index and returns a LabeledTile that contains the
// set of found traces. It also returns the subset of 'query' that contained
// valid parameters and values.
// If the returned query is empty the first return value is set to Nil,
// because now valid filter parameters were found in the query.
func (a *Analyzer) getSubTile(query map[string][]string) (*LabeledTile, map[string][]string) {
	traces, effectiveQuery := a.queryTraces(query)
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
func (a *Analyzer) getOutputCounts(labeledTile *LabeledTile) (*GUITileCounts, map[string]*GUITestCounts) {
	glog.Info("Starting to process output counts.")
	// Stores the aggregated counts of a tile for each test.
	tileCountsMap := make(map[string]*LabelCounts, len(labeledTile.Traces))

	// Stores the aggregated counts for each test and individual trace information.
	testCountsMap := make(map[string]*GUITestCounts, len(labeledTile.Traces))

	// Overall aggregated counts over all tests.
	overallAggregates := newLabelCounts(len(labeledTile.Commits))

	updateCounts(labeledTile, tileCountsMap, testCountsMap, overallAggregates)

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

	return tileCounts, testCountsMap
}

func updateCounts(labeledTile *LabeledTile, tileCountsMap map[string]*LabelCounts, testCountsMap map[string]*GUITestCounts, overallAggregates *LabelCounts) {
	for testName, testTraces := range labeledTile.Traces {
		acc := newLabelCounts(len(labeledTile.Commits))
		tempTraces := make([]*GUILabeledTrace, 0, len(testTraces))

		for _, oneTrace := range testTraces {
			tempTrace := &GUILabeledTrace{
				Params: oneTrace.Params,
				Labels: make([]IdLabel, len(oneTrace.CommitIds)),
			}

			for i, ci := range oneTrace.CommitIds {
				switch oneTrace.Labels[i] {
				case types.UNTRIAGED:
					acc.Unt[ci]++
				case types.POSITIVE:
					acc.Pos[ci]++
				case types.NEGATIVE:
					acc.Neg[ci]++
				}
				tempTrace.Labels[i].Id = ci
				tempTrace.Labels[i].Label = int(oneTrace.Labels[i])
			}

			tempTraces = append(tempTraces, tempTrace)
		}

		tileCountsMap[testName] = acc
		testCountsMap[testName] = &GUITestCounts{
			Commits:    labeledTile.Commits,
			Aggregated: acc,
			Traces:     tempTraces,
		}

		// Add the aggregates fro this test to the overall aggregates.
		for idx, u := range acc.Unt {
			overallAggregates.Unt[idx] += u
			overallAggregates.Pos[idx] += acc.Pos[idx]
			overallAggregates.Neg[idx] += acc.Neg[idx]
		}
	}
}
