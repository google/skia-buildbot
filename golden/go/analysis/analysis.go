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
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

// LabeledTrace stores a Trace with labels and digests. CommitIds, Digests and
// Labels are of the same length, identical indices refer to the same digest.
type LabeledTrace struct {
	Params    map[string]string
	CommitIds []int
	Digests   []string
	Labels    []types.Label
}

func NewLabeledTrace(params map[string]string, capacity int) *LabeledTrace {
	return &LabeledTrace{
		Params:    params,
		CommitIds: make([]int, 0, capacity),
		Digests:   make([]string, 0, capacity),
		Labels:    make([]types.Label, 0, capacity),
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
}

func NewLabeledTile() *LabeledTile {
	return &LabeledTile{
		Commits: []*ptypes.Commit{},
		Traces:  map[string][]*LabeledTrace{},
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
	newLT := NewLabeledTrace(params, trace.Len())
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
	Aggregated *LabelCounts            `json:"aggregated"`
	Counts     map[string]*LabelCounts `json:"counts"`
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
	currentTileCounts *GUITileCounts
	currentTestCounts map[string]*GUITestCounts

	// Lock to protect the expectations and current* variables.
	mutex sync.Mutex
}

func NewAnalyzer(expStore expstorage.ExpectationsStore, tileStore ptypes.TileStore, diffStore diff.DiffStore, timeBetweenPolls time.Duration) *Analyzer {
	result := &Analyzer{
		expStore:  expStore,
		diffStore: diffStore,
		tileStore: tileStore,

		currentTile: NewLabeledTile(),
	}

	go result.loop(timeBetweenPolls)
	return result
}

// GetTileCounts returns an entire Tile which is a collection of 'traces' over
// a series of commits. Each trace contains the digests and their labels
// based on our knowledge about digests (expectations).
func (a *Analyzer) GetTileCounts() (*GUITileCounts, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.currentTileCounts, nil
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
func (a *Analyzer) SetDigestLabels(labeledTestDigests map[string]types.TestClassification, userId string) (map[string][]*LabeledTrace, error) {
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
	updatedTraces := a.relabelTraces(labeledTestDigests)

	return updatedTraces, nil
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
		if err != nil {
			glog.Errorf("Error reading tile store: %s\n", err.Error())
			errorTileLoadingCounter.Inc(1)
		} else {
			newLabeledTile := a.processTile(tile)
			newTileCounts, newTestCounts := a.getOutputCounts(newLabeledTile)

			a.mutex.Lock()
			a.currentTile = newLabeledTile
			a.currentTileCounts = newTileCounts
			a.currentTestCounts = newTestCounts
			a.mutex.Unlock()
		}
		glog.Info("Done processing tiles.")
		runsCounter.Inc(1)
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

	return result
}

// relabelTraces iterates over the traces in of the tiles that have changed and
// labels them according to our current expecatations.
func (a *Analyzer) relabelTraces(labeledTestDigests map[string]types.TestClassification) map[string][]*LabeledTrace {
	result := map[string][]*LabeledTrace{}

	for testName := range labeledTestDigests {
		if traces, ok := a.currentTile.Traces[testName]; ok {
			for _, trace := range traces {
				// Note: This is potentially slower than using labels in
				// labeledTestDigests directly, but it keeps the code simpler.
				a.labelDigests(testName, trace.Digests, trace.Labels)
			}
			result[testName] = make([]*LabeledTrace, len(traces))
			copy(result[testName], traces)
		}
	}

	return result
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

// getOutputCounts derives the output counts from the given labeled tile.
func (a *Analyzer) getOutputCounts(labeledTile *LabeledTile) (*GUITileCounts, map[string]*GUITestCounts) {
	// Stores the aggregated counts of a tile for each test.
	tileCountsMap := make(map[string]*LabelCounts, len(labeledTile.Traces))

	// Stores the aggregated counts for each test and individual trace information.
	testCountsMap := make(map[string]*GUITestCounts, len(labeledTile.Traces))

	// Overall aggregated counts over all tests.
	overallAggregates := newLabelCounts(len(labeledTile.Commits))

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

	tileCounts := &GUITileCounts{
		Commits:    labeledTile.Commits,
		Aggregated: overallAggregates,
		Counts:     tileCountsMap,
	}

	return tileCounts, testCountsMap
}
