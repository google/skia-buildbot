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

// Stores a Trace with labels and digests in memory. CommitIds, Digests and
// Labels are of the same length, identical indices refer to the same digest.
type LabeledTrace struct {
	Params    map[string]string `json:"params"`
	CommitIds []int             `json:"commitIds"`
	Digests   []string          `json:"digests"`
	Labels    []types.Label     `json:"labels`
}

func NewLabeledTrace(params map[string]string, capacity int) *LabeledTrace {
	return &LabeledTrace{
		Params:    params,
		CommitIds: make([]int, 0, capacity),
		Digests:   make([]string, 0, capacity),
		Labels:    make([]types.Label, 0, capacity),
	}
}

// Add the given tripples of commitIds, digests and labels to this LabeledTrace.
func (lt *LabeledTrace) addLabeledDigests(commitIds []int, digests []string, labels []types.Label) {
	lt.CommitIds = append(lt.CommitIds, commitIds...)
	lt.Digests = append(lt.Digests, digests...)
	lt.Labels = append(lt.Labels, labels...)
}

// Aggregates the Traces in tile and provides the commits that the
// CommitIds in LabeledTrace refer to.
type LabeledTile struct {
	Commits []*ptypes.Commit `json:"commits"`

	// Traces are indexed by the primary key (test name). This is somewhat
	// redundant, but this also output format.
	Traces map[string][]*LabeledTrace `json:"traces"`
}

func NewLabeledTile() *LabeledTile {
	return &LabeledTile{
		Commits: []*ptypes.Commit{},
		Traces:  map[string][]*LabeledTrace{},
	}
}

// Utility function that returns the testName and a labeled trace for the given
// Trace (read from a TileStore). If the LabeledTrace does not exist it will be
// added.
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

// Analyzer continuously manages the tasks, like pollint for new traces
// on disk, etc.
type Analyzer struct {
	expStore  expstorage.ExpectationsStore
	diffStore diff.DiffStore
	tileStore ptypes.TileStore

	currentTile *LabeledTile

	// Lock to protect the expectations and the current labeled tile.
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

// Returns an entire Tile which is a collection of 'traces' over a series of
// of commits. Each trace contains the digests and their labels based on
// out knowledge base about digests (expectations).
func (a *Analyzer) GetLabeledTile() *LabeledTile {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.currentTile
}

func (a *Analyzer) GetLabeledTraces(testName string) []*LabeledTrace {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.currentTile.Traces[testName]
}

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

// Main loop.
func (a *Analyzer) loop(timeBetweenPolls time.Duration) {
	// The number of times we've successfully loaded and processed a tile.
	runsCounter := metrics.NewRegisteredCounter("analysis.runs", metrics.DefaultRegistry)

	// The number of times an error has ocurred when trying to load a tile.
	errorTileLoadingCounter := metrics.NewRegisteredCounter("analysis.errors", metrics.DefaultRegistry)

	for {
		glog.Info("Reading tiles ... ")

		// Load the tile and process it.
		tile, err := a.tileStore.Get(0, -1)
		if err != nil {
			glog.Errorf("Error reading tile store: %s\n", err.Error())
			errorTileLoadingCounter.Inc(1)
		} else {
			newLabeledTile := a.processTile(tile)
			a.mutex.Lock()
			a.currentTile = newLabeledTile
			a.mutex.Unlock()
		}
		runsCounter.Inc(1)

		// Sleep for a while until the next poll.
		time.Sleep(timeBetweenPolls)
	}
}

// Process a tile segment and add it to the currentTile.
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

// Run over the traces in of the tiles that have changed and label them
// according to our current expecatations.
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
