package analysis

import (
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/filediffstore"
	"go.skia.org/infra/golden/go/types"
	ptypes "go.skia.org/infra/perf/go/types"
)

const (
	NEW_TILE_CACHE_KEY     = "newtile"
	IGNORED_TILE_CACHE_KEY = "ignoredtile"

	// Number of commits that we are interested in.
	N_COMMITS = 50
)

var (
	// The number of times we've successfully loaded and processed a tile.
	runsCounter metrics.Counter

	// The number of times an error has ocurred when trying to load a tile.
	errorTileLoadingCounter metrics.Counter
)

func init() {
	runsCounter = metrics.NewRegisteredCounter("analysis.runs", metrics.DefaultRegistry)
	errorTileLoadingCounter = metrics.NewRegisteredCounter("analysis.errors", metrics.DefaultRegistry)
}

type PathToURLConverter func(string) string

// LabeledTrace stores a Trace with labels and digests. CommitIds, Digests and
// Labels are of the same length, identical indices refer to the same digest.
type LabeledTrace struct {
	Params      map[string]string
	CommitIds   []int
	Digests     []string
	Labels      []types.Label
	Id          int
	IgnoreRules []*types.IgnoreRule
}

func NewLabeledTrace(params map[string]string, capacity int, traceId int) *LabeledTrace {
	return &LabeledTrace{
		Params:      params,
		CommitIds:   make([]int, 0, capacity),
		Digests:     make([]string, 0, capacity),
		Labels:      make([]types.Label, 0, capacity),
		Id:          traceId,
		IgnoreRules: []*types.IgnoreRule{},
	}
}

// addLabledDigests adds the given tripples of commitIds, digests and labels to this LabeledTrace.
func (lt *LabeledTrace) addLabeledDigests(commitIds []int, digests []string, labels []types.Label) {
	lt.CommitIds = append(lt.CommitIds, commitIds...)
	lt.Digests = append(lt.Digests, digests...)
	lt.Labels = append(lt.Labels, labels...)
}

// addIgnoreRules attaches the ignore rules that match his trace.
func (lt *LabeledTrace) addIgnoreRules(newRules []*types.IgnoreRule) {
	lt.IgnoreRules = append(lt.IgnoreRules, newRules...)
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

	// CommitsByDigest maps a corpus and a digest to a list of commit ids.
	// i.e. CommitsByDigest[corpus][digest] -> slice with indices of Commits.
	CommitsByDigest map[string]map[string][]int

	// Keeps track of unique ids for traces within this tile.
	TraceIdCounter int
}

func NewLabeledTile() *LabeledTile {
	return &LabeledTile{
		Commits:         []*ptypes.Commit{},
		CommitsByDigest: map[string]map[string][]int{},
		Traces:          map[string][]*LabeledTrace{},
		TraceIdCounter:  0,
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
	newLT := NewLabeledTrace(params, trace.Len(), t.TraceIdCounter)
	t.TraceIdCounter++
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

// AnalyzeState captures the state of a partition of the incoming data.
// When a tile is read from disk it is partitioned into two tiles: current
// and ignored. current contains everything we want to be able to review
// continuously and ignored contains all ignored traces.
// This struct is the container for one of these partitions and the derived
// information.
type AnalyzeState struct {
	// Canonical data structure to hold our information about commits, digests
	// and labels.
	Tile *LabeledTile

	// Index to query the Tile.
	Index *LabeledTileIndex

	// Output data structures that are derived from Tile.
	TileCounts  *GUITileCounts
	TestDetails *GUITestDetails
	Status      *GUIStatus
	BlameLists  *GUIBlameLists
}

// Analyzer continuously manages tasks like polling for new traces
// on disk and generating diffs between images. It is the primary interface
// to be called by the HTTP frontend.
type Analyzer struct {
	expStore    expstorage.ExpectationsStore
	diffStore   diff.DiffStore
	tileStore   ptypes.TileStore
	ignoreStore types.IgnoreStore

	// current contains the state of the digests we are primariliy interested in.
	current *AnalyzeState

	// ignored contains the state of ignored digests.
	ignored *AnalyzeState

	// lastRawTile points to the last tile loaded from the tileStore.
	lastRawTile *ptypes.Tile

	// labeledTileCache caches the labeled tiles extracted tiles.
	labeledTileCache util.LRUCache

	// converter supplied by the client of the type to convert a path to a URL
	pathToURLConverter PathToURLConverter

	// Lock to protect the expectations and current* variables.
	mutex sync.RWMutex

	// Counts the number of times the main event loop has executed.
	// This is for testing only.
	loopCounter int
}

type LabeledTileCodec int

func (d LabeledTileCodec) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (d LabeledTileCodec) Decode(data []byte) (interface{}, error) {
	var v LabeledTile
	err := json.Unmarshal(data, &v)
	return &v, err
}

func NewAnalyzer(expStore expstorage.ExpectationsStore, tileStore ptypes.TileStore, diffStore diff.DiffStore, ignoreStore types.IgnoreStore, puConverter PathToURLConverter, cacheFactory filediffstore.CacheFactory, timeBetweenPolls time.Duration) *Analyzer {
	labeledTileCache := cacheFactory("ti", LabeledTileCodec(0))

	result := &Analyzer{
		expStore:           expStore,
		diffStore:          diffStore,
		tileStore:          tileStore,
		ignoreStore:        ignoreStore,
		pathToURLConverter: puConverter,

		current:          &AnalyzeState{},
		ignored:          &AnalyzeState{},
		lastRawTile:      nil,
		labeledTileCache: labeledTileCache,
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
			ret := a.getOutputCounts(tile, a.current.Index)
			ret.Query = effectiveQuery
			return ret, nil
		}
	}

	return a.current.TileCounts, nil
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
		return a.current.TestDetails, nil
	}

	effectiveQuery := make(map[string][]string, len(query))
	foundUntriaged := a.getUntriagedTestDetails(query, effectiveQuery, true)
	tests := make([]*GUITestDetail, 0, len(foundUntriaged))

	for testName, untriaged := range foundUntriaged {
		testDetail := a.current.TestDetails.lookup(testName)
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
		Commits:   a.current.TestDetails.Commits,
		AllParams: a.current.Index.getAllParams(query),
		Query:     effectiveQuery,
		Tests:     tests,
		Blames:    a.current.BlameLists.Blames,
	}, nil
}

func (a *Analyzer) ParamSet() (map[string][]string, error) {
	tile, err := a.tileStore.Get(0, -1)
	if err != nil {
		return nil, err
	}
	return tile.ParamSet, nil
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
	testDetail := a.current.TestDetails.lookup(testName)
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
		Commits:         a.current.TestDetails.Commits,
		CommitsByDigest: map[string]map[string][]int{testName: a.current.TestDetails.CommitsByDigest[testName]},
		AllParams:       a.current.Index.getAllParams(query),
		Query:           effectiveQuery,
		Blames:          map[string][]*BlameDistribution{testName: a.current.BlameLists.Blames[testName]},
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

	if err := a.expStore.AddChange(labeledTestDigests, userId); err != nil {
		return nil, err
	}

	expectations, err := a.expStore.Get()
	if err != nil {
		return nil, err
	}

	// Let's update our knowledge of the labels.
	a.updateDerivedOutputs(labeledTestDigests, expectations, a.current)
	a.updateDerivedOutputs(labeledTestDigests, expectations, a.ignored)

	result := make([]*GUITestDetail, 0, len(labeledTestDigests))
	for testName := range labeledTestDigests {
		result = append(result, a.current.TestDetails.lookup(testName))
	}

	return &GUITestDetails{
		Commits:   a.current.TestDetails.Commits,
		AllParams: a.current.Index.getAllParams(nil),
		Tests:     result,
	}, nil
}

func (a *Analyzer) GetStatus() *GUIStatus {
	return a.current.Status
}

// ListIgnoreRules returns all current ignore rules.
func (a *Analyzer) ListIgnoreRules() ([]*types.IgnoreRule, error) {
	rules, err := a.ignoreStore.List()
	if err != nil {
		return nil, err
	}

	// TODO(stephana): Inject Count and other statistics about the
	// ignored traces. This will be based on LabeledTrace.IgnoreRules.

	return rules, nil
}

// AddIgnoreRule adds a new ignore rule and recalculates the new state of the
// system.
func (a *Analyzer) AddIgnoreRule(ignoreRule *types.IgnoreRule) error {
	if err := a.ignoreStore.Create(ignoreRule); err != nil {
		return err
	}

	a.processTile(false, false)

	return nil
}

// DeleteIgnoreRule deletes the ignore rule and recalculates the state of the
// system.
func (a *Analyzer) DeleteIgnoreRule(ruleId int, user string) error {
	count, err := a.ignoreStore.Delete(ruleId, user)
	if err != nil {
		return err
	}

	if count > 0 {
		a.processTile(false, false)
	}

	return nil
}

// GetBlameList returns the blame lists for the test identified by testName or
// nil if there is no such test.
func (a *Analyzer) GetBlameList(testName string) *GUIBlameLists {
	ret, ok := a.current.BlameLists.Blames[testName]
	if !ok {
		return nil
	}

	return &GUIBlameLists{
		Commits: a.current.BlameLists.Commits,
		Blames:  map[string][]*BlameDistribution{testName: ret},
	}
}

// loop is the main event loop.
func (a *Analyzer) loop(timeBetweenPolls time.Duration) {
	// Process the tile with caching. If the result is false that means
	// no tile was loaded from disk and we need to do another run with
	// the latest tile.
	if !a.processTile(true, true) {
		a.processTile(false, true)
	}
	for _ = range time.Tick(timeBetweenPolls) {
		a.processTile(false, true)
	}
}

// getCachedTiles returns the last instances of current and ignored.
func (a *Analyzer) getCachedTiles() (*LabeledTile, *LabeledTile) {
	if newLabeledTile, ok := a.labeledTileCache.Get(NEW_TILE_CACHE_KEY); ok {
		if ignoredLabeledTile, ok := a.labeledTileCache.Get(IGNORED_TILE_CACHE_KEY); ok {
			return newLabeledTile.(*LabeledTile), ignoredLabeledTile.(*LabeledTile)
		}
	}
	return nil, nil
}

// cacheTiles stores the latest instances of current and ignored.
func (a *Analyzer) cacheTiles(newLabeledTile *LabeledTile, ignoredLabeledTile *LabeledTile) {
	a.labeledTileCache.Add(NEW_TILE_CACHE_KEY, newLabeledTile)
	a.labeledTileCache.Add(IGNORED_TILE_CACHE_KEY, ignoredLabeledTile)
}

// processTile loads a tile (built by the ingest process) and partitions it
// into two labeled tiles one with the traces of interest and the traces we
// are ignoring.
// The flags indicate whether to use the cached labeled tiles (during startup)
// and whether to reload the tile from disk or use lastRawTile instead.
// The return value indicates whether a raw tile was loaded from disk.
func (a *Analyzer) processTile(useCached bool, reloadRawTile bool) bool {
	loadedTile := false
	defer timer.New("processTile").Stop()
	glog.Infof("Starting processtile: %t - %t", useCached, reloadRawTile)

	var newLabeledTile, ignoredLabeledTile *LabeledTile = nil, nil
	var err error

	// Load the labeled tiles from cache. This will require no new diffs
	// since they have already been calculated for the cached tile.
	if useCached {
		newLabeledTile, ignoredLabeledTile = a.getCachedTiles()
	}

	// If we don't have labeled tiles at this point we need to get a raw tile
	// and build a labeled tile.
	if newLabeledTile == nil {
		// Either use a tile already in memory or load a new one.
		var tile *ptypes.Tile
		if reloadRawTile || (a.lastRawTile == nil) {
			tile, err = a.tileStore.Get(0, -1)
			if err != nil {
				glog.Errorf("Error reading tile store: %s\n", err)
				errorTileLoadingCounter.Inc(1)
				return false
			}
			tileLen := tile.LastCommitIndex() + 1
			tile, err = tile.Trim(util.MaxInt(0, tileLen-N_COMMITS), tileLen)
			if err != nil {
				glog.Errorf("Error trimming tile: %s\n", err)
				return false
			}
			glog.Infof("TileLen: %d", tile.LastCommitIndex()+1)

			a.lastRawTile = tile
			loadedTile = true
			glog.Infoln("Loaded new tile from disk.")
		} else {
			tile = a.lastRawTile
			glog.Infoln("Reusing last raw tile.")
		}

		newLabeledTile, ignoredLabeledTile = a.partitionRawTile(tile)

		a.prepDiffsForLabeledTile(newLabeledTile)
		a.prepDiffsForLabeledTile(ignoredLabeledTile)
		a.cacheTiles(newLabeledTile, ignoredLabeledTile)
	}

	glog.Infof("Tests in newLabeledTiles    : %d", len(newLabeledTile.Traces))
	glog.Infof("Tests in ignoredLabeledTiles: %d", len(ignoredLabeledTile.Traces))

	// Protect the tile and expectations with the write lock.
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Retrieve the current expectations.
	expectations, err := a.expStore.Get()
	if err != nil {
		glog.Errorf("Error retrieving expectations: %s", err)
		return false
	}
	a.setDerivedOutputs(newLabeledTile, expectations, a.current, false)
	a.setDerivedOutputs(ignoredLabeledTile, expectations, a.ignored, false)

	glog.Info("Done processing tiles.")
	runsCounter.Inc(1)
	a.loopCounter++

	return loadedTile
}

// completeDiffs forces the DiffStore to precalculate the diffs
// between all digests (within a test).
// TODO(stephana): This is currently not used, but will be enabled once
// the filediffstore can efficiently handle complete diffs.
func (a *Analyzer) completeDiffs(labeledTile *LabeledTile) {
	var wg sync.WaitGroup
	for _, traces := range labeledTile.Traces {
		wg.Add(1)
		go func(traces []*LabeledTrace) {
			digestsList := make([][]string, 0, len(traces))
			for _, t := range traces {
				digestsList = append(digestsList, t.Digests)
			}
			allDigests := util.UnionStrings(digestsList...)
			a.diffStore.CalculateDiffs(allDigests)
			wg.Done()
		}(traces)
	}

	wg.Wait()
}

// prepDiffsForLabeledTile forces the DiffStore to precalculate the diffs
// between new digests. We do this outside the locking so that when we are
// inside the lock almost all diffs will be cached already.
func (a *Analyzer) prepDiffsForLabeledTile(labeledTile *LabeledTile) {
	glog.Infof("Starting prep diffs")
	// Get the current expectations.
	a.mutex.RLock()
	expectations, err := a.expStore.Get()
	a.mutex.RUnlock()
	if err != nil {
		glog.Errorf("Unable to read expectations: %s", err)
		return
	}

	// Make a dummy call to setDerivedOutputs to force a diff on new
	// digest pairs.
	tempState := AnalyzeState{}
	a.setDerivedOutputs(labeledTile, expectations, &tempState, true)
	glog.Infof("Done prep diffs")
}

// partitionRawTile partitions the input tile into two tiles (current and ignored)
// and derives the output data structures for both.
func (a *Analyzer) partitionRawTile(tile *ptypes.Tile) (*LabeledTile, *LabeledTile) {
	glog.Info("Processing tile into LabeledTile ...")

	// Shared between both tiles.
	tileLen := tile.LastCommitIndex() + 1

	// Set the up the result tile and a tile for ignored traces.
	resultTile := NewLabeledTile()
	resultTile.Commits = tile.Commits[:tileLen]
	resultCommitsByDigestMap := map[string]map[string]map[int]bool{}

	ignoredTile := NewLabeledTile()
	ignoredTile.Commits = tile.Commits[:tileLen]
	ignoredCommitsByDigestMap := map[string]map[string]map[int]bool{}

	// Get the digests that are unavailable, e.g. they cannot be fetched
	// from GS or they are not valid images.
	unavailableDigests := a.diffStore.UnavailableDigests()
	glog.Infof("Unavailable digests: %v", unavailableDigests)

	// Get the rule matcher to find traces to ignore.
	ruleMatcher, err := a.ignoreStore.BuildRuleMatcher()
	if err != nil {
		glog.Errorf("Unable to build rule matcher: %s", err)
	}

	// Note: We are assumming that the number and order of traces will change
	// over time.
	var targetTile *LabeledTile
	var commitsByDigestMap map[string]map[string]map[int]bool
	for _, v := range tile.Traces {
		// Determine if this tile is to be in the result or the ignored tile.
		matchedRules, isIgnored := ruleMatcher(v.Params())
		if isIgnored {
			targetTile = ignoredTile
			commitsByDigestMap = ignoredCommitsByDigestMap
		} else {
			targetTile = resultTile
			commitsByDigestMap = resultCommitsByDigestMap
		}

		tempCommitIds := make([]int, 0, tileLen)
		tempLabels := make([]types.Label, 0, tileLen)
		tempDigests := make([]string, 0, tileLen)
		gTrace := v.(*ptypes.GoldenTrace)
		testName := gTrace.Params()[types.PRIMARY_KEY_FIELD]

		// Iterate over the digests in this trace.
		for i, v := range gTrace.Values[:tileLen] {
			if (v != ptypes.MISSING_DIGEST) && !unavailableDigests[v] {
				tempCommitIds = append(tempCommitIds, i)
				tempDigests = append(tempDigests, v)
				tempLabels = append(tempLabels, types.UNTRIAGED)

				// Keep track of the commits by digest.
				if _, ok := commitsByDigestMap[testName]; !ok {
					commitsByDigestMap[testName] = map[string]map[int]bool{v: map[int]bool{i: true}}
				} else if _, ok := commitsByDigestMap[testName][v]; !ok {
					commitsByDigestMap[testName][v] = map[int]bool{i: true}
				} else {
					commitsByDigestMap[testName][v][i] = true
				}
			}
		}

		// Only consider traces that are not empty.
		if len(tempLabels) > 0 {
			// Label the digests and add them to the labeled traces.
			_, targetLabeledTrace := targetTile.getLabeledTrace(v)
			targetLabeledTrace.addLabeledDigests(tempCommitIds, tempDigests, tempLabels)
			if isIgnored {
				targetLabeledTrace.addIgnoreRules(matchedRules)
			}
		}
	}

	getCommitsByDigest(resultTile, resultCommitsByDigestMap)
	getCommitsByDigest(ignoredTile, ignoredCommitsByDigestMap)

	glog.Info("Done processing tile into LabeledTile.")
	return resultTile, ignoredTile
}

func getCommitsByDigest(labeledTile *LabeledTile, commitsByDigestMap map[string]map[string]map[int]bool) {
	for testName, cbd := range commitsByDigestMap {
		labeledTile.CommitsByDigest[testName] = make(map[string][]int, len(cbd))
		for d, commitIds := range cbd {
			labeledTile.CommitsByDigest[testName][d] = util.KeysOfIntSet(commitIds)
			sort.Ints(labeledTile.CommitsByDigest[testName][d])
		}
	}
}

// setDerivedOutputs derives the output data from the given tile and
// updates the outputs and tile in the analyzer.
func (a *Analyzer) setDerivedOutputs(labeledTile *LabeledTile, expectations *expstorage.Expectations, state *AnalyzeState, prep bool) {
	// Assign all the labels.
	for testName, traces := range labeledTile.Traces {
		for _, trace := range traces {
			labelDigests(testName, trace.Digests, trace.Labels, expectations)
		}
	}

	// Generate the lookup index for the tile and get all parameters.
	state.Index = NewLabeledTileIndex(labeledTile)
	state.Tile = labeledTile
	state.TestDetails = a.getTestDetails(state)

	// Don't calculate these during prep runs.
	if !prep {
		state.TileCounts = a.getOutputCounts(state.Tile, state.Index)
		state.Status = calcStatus(state)
		state.BlameLists = getBlameLists(state.Tile)
	}
}

// updateLabels iterates over the traces in of the tiles that have changed and
// labels them according to our current expecatations.

// updateDerivedOutputs
func (a *Analyzer) updateDerivedOutputs(labeledTestDigests map[string]types.TestClassification, expectations *expstorage.Expectations, state *AnalyzeState) {
	// Update the labels of the traces that have changed.
	for testName := range labeledTestDigests {
		if traces, ok := state.Tile.Traces[testName]; ok {
			for _, trace := range traces {
				// Note: This is potentially slower than using labels in
				// labeledTestDigests directly, but it keeps the code simpler.
				labelDigests(testName, trace.Digests, trace.Labels, expectations)
			}
		}
	}

	// Update all the output data structures.
	// TODO(stephana): Evaluate whether the counts are really useful or if they can be removed.
	// If we need them uncomment the following line and implement the corresponding function.
	//a.updateOutputCounts(labeledTestDigests)

	// Update the tests that have changed and the status.
	a.updateTestDetails(labeledTestDigests, state)
	state.Status = calcStatus(state)
}

// labelDigest assignes a label to the given digests based on the expectations.
// Its assumes that targetLabels are pre-initialized, usualy with UNTRIAGED,
// because it will not change the label if the given test and digest cannot be
// found.
func labelDigests(testName string, digests []string, targetLabels []types.Label, expectations *expstorage.Expectations) {
	for idx, digest := range digests {
		if test, ok := expectations.Tests[testName]; ok {
			if foundLabel, ok := test[digest]; ok {
				targetLabels[idx] = foundLabel
			}
		}
	}
}

// getUntriagedTestDetails returns the untriaged digests of a specific test that
// match the given query. In addition to the digests it returns the query
// that was used to retrieve them.
func (a *Analyzer) getUntriagedTestDetails(query, effectiveQuery map[string][]string, includeAllTests bool) map[string]map[string]*GUIUntriagedDigest {
	traces, startCommitId, endCommitId, showHead := a.current.Index.query(query, effectiveQuery)
	endCommitId++

	if len(effectiveQuery) == 0 {
		return nil
	}

	ret := make(map[string]map[string]*GUIUntriagedDigest, len(a.current.TestDetails.Tests))

	// This includes an empty list for tests that we have not found.
	if includeAllTests {
		for _, testName := range a.current.Index.getTestNames(query) {
			ret[testName] = nil
		}
	}

	if !showHead {
		for _, trace := range traces {
			testName := trace.Params[types.PRIMARY_KEY_FIELD]
			current := a.current.TestDetails.lookup(testName).Untriaged

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
		}
	} else {
		for _, trace := range traces {
			lastIdx := len(trace.Labels) - 1
			if (lastIdx >= 0) && (trace.Labels[lastIdx] == types.UNTRIAGED) {
				testName := trace.Params[types.PRIMARY_KEY_FIELD]
				current := a.current.TestDetails.lookup(testName).Untriaged
				if found, ok := ret[testName]; !ok || found == nil {
					ret[testName] = map[string]*GUIUntriagedDigest{}
				}
				ret[testName][trace.Digests[lastIdx]] = current[trace.Digests[lastIdx]]
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
	traces, _, _, _ := a.current.Index.query(query, effectiveQuery)
	if len(effectiveQuery) == 0 {
		return nil, effectiveQuery
	}

	result := NewLabeledTile()
	result.Commits = a.current.Tile.Commits

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
func (a *Analyzer) getOutputCounts(labeledTile *LabeledTile, index *LabeledTileIndex) *GUITileCounts {
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
		AllParams:  index.getAllParams(nil),
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
