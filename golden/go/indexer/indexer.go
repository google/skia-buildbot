// Package indexer continuously creates an index of the test results
// as the tiles, expectations, and ignores change.
package indexer

import (
	"context"
	"fmt"
	"sync"
	"time"

	ttlcache "github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/digest_counter"
	"go.skia.org/infra/golden/go/digesttools"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/pdag"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tilesource"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
)

const (
	// Metric to track the number of digests that do not have be uploaded by bots.
	knownHashesMetric = "known_digests"
	// Metric to track the number of changelists we currently have indexed.
	indexedCLsMetric = "gold_indexed_changelists"

	reindexedCLsMetric = "gold_indexer_changelists_reindexed"
)

// SearchIndex contains everything that is necessary to search
// our current knowledge about test results. It should be
// considered as immutable. Whenever the underlying data changes,
// a new index is calculated via a pdag.
type SearchIndex struct {
	searchIndexConfig
	// The indices of these arrays are the int values of types.IgnoreState
	dCounters         [2]digest_counter.DigestCounter
	summaries         [2]countsAndBlames
	paramsetSummaries [2]paramsets.ParamSummary
	preSliced         map[preSliceGroup][]*tiling.TracePair

	cpxTile tiling.ComplexTile
	blamer  blame.Blamer

	// This is set by the indexing pipeline when we just want to update
	// individual tests that have changed.
	testNames types.TestNameSet
}

type preSliceGroup struct {
	IgnoreState types.IgnoreState
	Corpus      string
	Test        types.TestName
}

// countsAndBlame makes the type declaration of SearchIndex a little nicer to read.
type countsAndBlames []*summary.TriageStatus

type searchIndexConfig struct {
	diffStore         diff.DiffStore
	expectationsStore expectations.Store
	gcsClient         storage.GCSClient
	warmer            warmer.DiffWarmer
}

// newSearchIndex creates a new instance of SearchIndex. It is not intended to
// be used outside of this package. SearchIndex instances are created by the
// Indexer and retrieved via GetIndex().
func newSearchIndex(sic searchIndexConfig, cpxTile tiling.ComplexTile) *SearchIndex {
	return &SearchIndex{
		searchIndexConfig: sic,
		// The indices of these slices are the int values of types.IgnoreState
		dCounters:         [2]digest_counter.DigestCounter{},
		summaries:         [2]countsAndBlames{},
		paramsetSummaries: [2]paramsets.ParamSummary{},
		preSliced:         map[preSliceGroup][]*tiling.TracePair{},
		cpxTile:           cpxTile,
	}
}

// SearchIndexForTesting returns filled in search index to be used when testing. Note that the
// indices of the arrays are the int values of types.IgnoreState
func SearchIndexForTesting(cpxTile tiling.ComplexTile, dc [2]digest_counter.DigestCounter, pm [2]paramsets.ParamSummary, exp expectations.Store, b blame.Blamer) (*SearchIndex, error) {
	s := &SearchIndex{
		searchIndexConfig: searchIndexConfig{
			expectationsStore: exp,
		},
		dCounters:         dc,
		summaries:         [2]countsAndBlames{},
		paramsetSummaries: pm,
		preSliced:         map[preSliceGroup][]*tiling.TracePair{},
		blamer:            b,
		cpxTile:           cpxTile,
	}
	// Providing the context.Background here is ok because this function is only going to be
	// called from tests.
	return s, preSliceData(context.Background(), s)
}

// Tile implements the IndexSearcher interface.
func (idx *SearchIndex) Tile() tiling.ComplexTile {
	return idx.cpxTile
}

// GetIgnoreMatcher implements the IndexSearcher interface.
func (idx *SearchIndex) GetIgnoreMatcher() paramtools.ParamMatcher {
	return idx.cpxTile.IgnoreRules()
}

// DigestCountsByTest implements the IndexSearcher interface.
func (idx *SearchIndex) DigestCountsByTest(is types.IgnoreState) map[types.TestName]digest_counter.DigestCount {
	return idx.dCounters[is].ByTest()
}

// MaxDigestsByTest implements the IndexSearcher interface.
func (idx *SearchIndex) MaxDigestsByTest(is types.IgnoreState) map[types.TestName]types.DigestSet {
	return idx.dCounters[is].MaxDigestsByTest()
}

// DigestCountsByTrace implements the IndexSearcher interface.
func (idx *SearchIndex) DigestCountsByTrace(is types.IgnoreState) map[tiling.TraceID]digest_counter.DigestCount {
	return idx.dCounters[is].ByTrace()
}

// DigestCountsByQuery implements the IndexSearcher interface.
func (idx *SearchIndex) DigestCountsByQuery(query paramtools.ParamSet, is types.IgnoreState) digest_counter.DigestCount {
	return idx.dCounters[is].ByQuery(idx.cpxTile.GetTile(is), query)
}

// GetSummaries implements the IndexSearcher interface.
func (idx *SearchIndex) GetSummaries(is types.IgnoreState) []*summary.TriageStatus {
	return idx.summaries[is]
}

// SummarizeByGrouping implements the IndexSearcher interface.
func (idx *SearchIndex) SummarizeByGrouping(ctx context.Context, corpus string, query paramtools.ParamSet, is types.IgnoreState, head bool) ([]*summary.TriageStatus, error) {
	exp, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The summaries are broken down by grouping (currently corpus and test name). Conveniently,
	// we already have the traces broken down by those areas, and summaries are independent, so we
	// can calculate them in parallel.
	type groupedTracePairs []*tiling.TracePair
	var groups []groupedTracePairs
	for g, traces := range idx.preSliced {
		if g.IgnoreState == is && g.Corpus == corpus && g.Test != "" {
			groups = append(groups, traces)
		}
	}
	rv := make([]*summary.TriageStatus, len(groups))
	wg := sync.WaitGroup{}
	for i, g := range groups {
		wg.Add(1)
		go func(slice int, gtp groupedTracePairs) {
			defer wg.Done()
			d := summary.Data{
				Traces: gtp,
				// These are all thread-safe, so they can be shared.
				Expectations: exp,
				ByTrace:      idx.dCounters[is].ByTrace(),
				Blamer:       idx.blamer,
			}
			ts := d.Calculate(nil, query, head)
			if len(ts) > 1 {
				// this should never happen, as we'd only get multiple if there were multiple
				// tests in the pre-sliced data (e.g. our pre-slicing code is bugged).
				sklog.Warningf("Summary Calculation should always be length 1, but wasn't %#v", ts)
				return
			} else if len(ts) == 0 {
				// This will happen if query removes all of the traces belonging to this test.
				// It results in a nil in the return value; if that is a problem we can either
				// fill in a zeroish value or a TriageStatus with Test/Corpus filled and 0 in the
				// counts.
				return
			}
			rv[slice] = ts[0]
		}(i, g)
	}
	wg.Wait()
	return rv, nil
}

// GetParamsetSummary implements the IndexSearcher interface.
func (idx *SearchIndex) GetParamsetSummary(test types.TestName, digest types.Digest, is types.IgnoreState) paramtools.ParamSet {
	return idx.paramsetSummaries[is].Get(test, digest)
}

// GetParamsetSummaryByTest implements the IndexSearcher interface.
func (idx *SearchIndex) GetParamsetSummaryByTest(is types.IgnoreState) map[types.TestName]map[types.Digest]paramtools.ParamSet {
	return idx.paramsetSummaries[is].GetByTest()
}

// GetBlame implements the IndexSearcher interface.
func (idx *SearchIndex) GetBlame(test types.TestName, digest types.Digest, commits []tiling.Commit) blame.BlameDistribution {
	if idx.blamer == nil {
		// should never happen - indexer should have this initialized
		// before the web server starts serving requests.
		return blame.BlameDistribution{}
	}
	return idx.blamer.GetBlame(test, digest, commits)
}

// SlicedTraces returns a slice of TracePairs that match the query and the ignore state.
// This is meant to be a superset of traces, as only the corpus and testname from the query are
// used for this pre-filter step.
func (idx *SearchIndex) SlicedTraces(is types.IgnoreState, query map[string][]string) []*tiling.TracePair {
	if len(query[types.CorpusField]) == 0 {
		return idx.preSliced[preSliceGroup{
			IgnoreState: is,
		}]
	}
	var rv []*tiling.TracePair
	for _, corpus := range query[types.CorpusField] {
		if len(query[types.PrimaryKeyField]) == 0 {
			rv = append(rv, idx.preSliced[preSliceGroup{
				IgnoreState: is,
				Corpus:      corpus,
			}]...)
		} else {
			for _, tn := range query[types.PrimaryKeyField] {
				rv = append(rv, idx.preSliced[preSliceGroup{
					IgnoreState: is,
					Corpus:      corpus,
					Test:        types.TestName(tn),
				}]...)
			}
		}
	}
	return rv
}

// MostRecentPositiveDigest implements the IndexSearcher interface.
func (idx *SearchIndex) MostRecentPositiveDigest(ctx context.Context, traceID tiling.TraceID) (types.Digest, error) {
	defer metrics2.FuncTimer().Stop()

	// Retrieve Trace for the given traceID.
	tr, ok := idx.cpxTile.GetTile(types.IncludeIgnoredTraces).Traces[traceID]
	if !ok {
		return tiling.MissingDigest, nil
	}

	// Retrieve expectations.
	exps, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "retrieving expectations (traceID=%q)", traceID)
	}

	// Find and return the most recent positive digest in the Trace.
	for i := len(tr.Digests) - 1; i >= 0; i-- {
		digest := tr.Digests[i]
		if digest != tiling.MissingDigest && exps.Classification(tr.TestName(), digest) == expectations.Positive {
			return digest, nil
		}
	}
	return tiling.MissingDigest, nil
}

type IndexerConfig struct {
	ExpChangeListener expectations.ChangeEventRegisterer
	DiffWorkPublisher diff.Calculator
	DiffStore         diff.DiffStore
	ExpectationsStore expectations.Store
	GCSClient         storage.GCSClient
	ReviewSystems     []clstore.ReviewSystem
	TileSource        tilesource.TileSource
	TryJobStore       tjstore.Store
	Warmer            warmer.DiffWarmer
}

// Indexer is the type that continuously processes data as the underlying
// data change. It uses a DAG that encodes the dependencies of the
// different components of an index and creates a processing pipeline on top
// of it.
type Indexer struct {
	IndexerConfig

	pipeline         *pdag.Node
	indexTestsNode   *pdag.Node
	lastMasterIndex  *SearchIndex
	masterIndexMutex sync.RWMutex

	changelistIndices *ttlcache.Cache

	changelistsReindexed metrics2.Counter
}

// New returns a new IndexSource instance. It synchronously indexes the initially
// available tile. If the indexing fails an error is returned.
// The provided interval defines how often the index should be refreshed.
func New(ctx context.Context, ic IndexerConfig, interval time.Duration) (*Indexer, error) {
	ret := &Indexer{
		IndexerConfig:        ic,
		changelistIndices:    ttlcache.New(changelistCacheExpirationDuration, changelistCacheExpirationDuration),
		changelistsReindexed: metrics2.GetCounter(reindexedCLsMetric),
	}

	// Set up the processing pipeline.
	root := pdag.NewNodeWithParents(pdag.NoOp)

	// At the top level, Add the DigestCounters...
	countsNodeInclude := root.Child(calcDigestCountsInclude)
	// These are run in parallel because they can take tens of seconds
	// in large repos like Skia.
	countsNodeExclude := root.Child(calcDigestCountsExclude)

	// This can run in parallel because it's just counting and sending data to Pub/Sub.
	root.Child(ret.sendWorkToDiffCalculators)

	preSliceNode := root.Child(preSliceData)

	// Node that triggers blame and writing baselines.
	// This is used to trigger when expectations change.
	// We don't need to re-calculate DigestCounts if the
	// expectations change because the DigestCounts don't care about
	// the expectations, only on the tile.
	indexTestsNode := root.Child(pdag.NoOp)

	// ... and invoke the Blamer to calculate the blames.
	blamerNode := indexTestsNode.Child(calcBlame)

	// Parameters depend on DigestCounter.
	paramsNodeInclude := pdag.NewNodeWithParents(calcParamsetsInclude, countsNodeInclude)
	// These are run in parallel because they can take tens of seconds
	// in large repos like Skia.
	paramsNodeExclude := pdag.NewNodeWithParents(calcParamsetsExclude, countsNodeExclude)

	// Write known hashes after ignores are computed. DigestCount is a
	// convenient way to get all the hashes, so that's what this node uses.
	writeHashes := countsNodeInclude.Child(writeKnownHashesList)

	// Summaries depend on DigestCounter and Blamer.
	summariesNode := pdag.NewNodeWithParents(calcSummaries, countsNodeInclude, countsNodeExclude, blamerNode, preSliceNode)

	// The Warmer depends on summaries.
	pdag.NewNodeWithParents(runWarmer, summariesNode)

	// Set the result on the Indexer instance, once summaries, parameters and writing
	// the hash files is done.
	pdag.NewNodeWithParents(ret.setIndex, summariesNode, paramsNodeInclude, paramsNodeExclude, writeHashes)

	ret.pipeline = root
	ret.indexTestsNode = indexTestsNode

	// Process the first tile and start the indexing process.
	return ret, ret.start(ctx, interval)
}

// GetIndex implements the IndexSource interface.
func (ix *Indexer) GetIndex() IndexSearcher {
	return ix.getIndex()
}

// getIndex is like GetIndex but returns the bare struct, for
// internal package use.
func (ix *Indexer) getIndex() *SearchIndex {
	ix.masterIndexMutex.RLock()
	defer ix.masterIndexMutex.RUnlock()
	return ix.lastMasterIndex
}

// start builds the initial index and starts the background
// process to continuously build indices.
func (ix *Indexer) start(ctx context.Context, interval time.Duration) error {
	if interval == 0 {
		sklog.Warning("Not starting indexer because duration was 0")
		return nil
	}

	defer shared.NewMetricsTimer("initial_synchronous_index").Stop()
	// Build the first index synchronously.
	tileStream := tilesource.GetTileStreamNow(ix.TileSource, interval, "gold-indexer")
	if err := ix.executePipeline(ctx, <-tileStream); err != nil {
		return skerr.Wrap(err)
	}

	// When the master expectations change, update the blamer and its dependents. This channel
	// will usually be empty, except when triaging happens. We set the size to be big enough to
	// handle a large bulk triage, if needed.
	expCh := make(chan expectations.ID, 100000)
	ix.ExpChangeListener.ListenForChange(func(e expectations.ID) {
		// Schedule the list of test names to be recalculated.
		expCh <- e
	})

	// Keep building indices for different types of events. This is the central
	// event loop of the indexer.
	go func() {
		var cpxTile tiling.ComplexTile
		for {
			if err := ctx.Err(); err != nil {
				sklog.Warningf("Stopping indexer - context error: %s", err)
				return
			}
			var testChanges []expectations.ID

			// See if there is a tile or changed tests.
			cpxTile = nil
			select {
			// Catch a new tile.
			case cpxTile = <-tileStream:
				sklog.Infof("Indexer saw a new tile")

				// Catch any test changes.
			case tn := <-expCh:
				testChanges = append(testChanges, tn)
				sklog.Infof("Indexer saw some tests change")
			}

			// Drain all input channels, effectively bunching signals together that arrive in short
			// succession.
		DrainLoop:
			for {
				select {
				case tn := <-expCh:
					testChanges = append(testChanges, tn)
				default:
					break DrainLoop
				}
			}

			// If there is a tile, re-index everything and forget the
			// individual tests that changed.
			if cpxTile != nil {
				if err := ix.executePipeline(ctx, cpxTile); err != nil {
					sklog.Errorf("Unable to index tile: %s", err)
				}
			} else if len(testChanges) > 0 {
				// Only index the tests that have changed.
				ix.indexTests(ctx, testChanges)
			}
		}
	}()

	// Start indexing the CLs now that the first index has been populated (we need the
	// primary branch index to get the digests to diff against).
	go util.RepeatCtx(ctx, interval, ix.calcChangelistIndices)

	return nil
}

// executePipeline runs the given tile through the the indexing pipeline.
// pipeline.Trigger blocks until everything is done, so this function will as well.
func (ix *Indexer) executePipeline(ctx context.Context, cpxTile tiling.ComplexTile) error {
	defer shared.NewMetricsTimer("indexer_execute_pipeline").Stop()
	// Create a new index from the given tile.
	sic := searchIndexConfig{
		diffStore:         ix.DiffStore,
		expectationsStore: ix.ExpectationsStore,
		gcsClient:         ix.GCSClient,
		warmer:            ix.Warmer,
	}
	return ix.pipeline.Trigger(ctx, newSearchIndex(sic, cpxTile))
}

// indexTest creates an updated index by indexing the given list of expectation changes.
func (ix *Indexer) indexTests(ctx context.Context, testChanges []expectations.ID) {
	// Get all the test names that had expectations changed.
	testNames := types.TestNameSet{}
	for _, d := range testChanges {
		testNames[d.Grouping] = true
	}
	if len(testNames) == 0 {
		return
	}

	sklog.Infof("Going to re-index %d tests", len(testNames))

	defer shared.NewMetricsTimer("index_tests").Stop()
	newIdx := ix.cloneLastIndex()
	// Set the testNames such that we only recompute those tests.
	newIdx.testNames = testNames
	if err := ix.indexTestsNode.Trigger(ctx, newIdx); err != nil {
		sklog.Errorf("Error indexing tests: %v \n\n Got error: %s", testNames.Keys(), err)
	}
}

// cloneLastIndex returns a copy of the most recent index.
func (ix *Indexer) cloneLastIndex() *SearchIndex {
	lastIdx := ix.getIndex()
	sic := searchIndexConfig{
		diffStore:         ix.DiffStore,
		expectationsStore: ix.ExpectationsStore,
		gcsClient:         ix.GCSClient,
		warmer:            ix.Warmer,
	}
	return &SearchIndex{
		searchIndexConfig: sic,
		cpxTile:           lastIdx.cpxTile,
		dCounters:         lastIdx.dCounters,         // stay the same even if expectations change.
		paramsetSummaries: lastIdx.paramsetSummaries, // stay the same even if expectations change.
		preSliced:         lastIdx.preSliced,         // stay the same even if expectations change.

		summaries: [2]countsAndBlames{
			// the objects inside the summaries are immutable, but may be replaced if expectations
			// are recalculated for a subset of tests.
			lastIdx.summaries[types.ExcludeIgnoredTraces],
			lastIdx.summaries[types.IncludeIgnoredTraces],
		},

		blamer: nil, // This will need to be recomputed if expectations change.

		// Force testNames to be empty, just to be sure we re-compute everything by default
		testNames: nil,
	}
}

// setIndex sets the lastMasterIndex value at the very end of the pipeline.
func (ix *Indexer) setIndex(_ context.Context, state interface{}) error {
	newIndex := state.(*SearchIndex)
	ix.masterIndexMutex.Lock()
	defer ix.masterIndexMutex.Unlock()
	ix.lastMasterIndex = newIndex
	return nil
}

// calcDigestCountsInclude is the pipeline function to calculate DigestCounts from
// the full tile (not applying ignore rules)
func calcDigestCountsInclude(_ context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	is := types.IncludeIgnoredTraces
	idx.dCounters[is] = digest_counter.New(idx.cpxTile.GetTile(is))
	return nil
}

// calcDigestCountsExclude is the pipeline function to calculate DigestCounts from
// the partial tile (applying ignore rules).
func calcDigestCountsExclude(_ context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	is := types.ExcludeIgnoredTraces
	idx.dCounters[is] = digest_counter.New(idx.cpxTile.GetTile(is))
	return nil
}

// preSliceData is the pipeline function to pre-slice our traces. Currently, we pre-slice by
// corpus name and then by test name because this breaks our traces up into groups of <1000.
func preSliceData(_ context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	for _, is := range types.IgnoreStates {
		t := idx.cpxTile.GetTile(is)
		for id, tr := range t.Traces {
			if tr == nil {
				sklog.Warningf("Unexpected nil trace id %s", id)
				continue
			}
			tp := tiling.TracePair{
				ID:    id,
				Trace: tr,
			}
			// Pre-slice the data by IgnoreState, then by IgnoreState and Corpus, finally by all
			// three of IgnoreState/Corpus/Test. We shouldn't allow queries by Corpus w/o specifying
			// IgnoreState, nor should we allow queries by TestName w/o specifying a Corpus or
			// IgnoreState.
			ignoreOnly := preSliceGroup{
				IgnoreState: is,
			}
			idx.preSliced[ignoreOnly] = append(idx.preSliced[ignoreOnly], &tp)

			ignoreAndCorpus := preSliceGroup{
				IgnoreState: is,
				Corpus:      tr.Corpus(),
			}
			idx.preSliced[ignoreAndCorpus] = append(idx.preSliced[ignoreAndCorpus], &tp)

			ignoreCorpusTest := preSliceGroup{
				IgnoreState: is,
				Corpus:      tr.Corpus(),
				Test:        tr.TestName(),
			}
			idx.preSliced[ignoreCorpusTest] = append(idx.preSliced[ignoreCorpusTest], &tp)
		}
	}
	return nil
}

// calcSummaries is the pipeline function to calculate the summaries.
func calcSummaries(ctx context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	exp, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, is := range types.IgnoreStates {
		d := summary.Data{
			Traces:       idx.SlicedTraces(is, nil),
			Expectations: exp,
			ByTrace:      idx.dCounters[is].ByTrace(),
			Blamer:       idx.blamer,
		}
		sum := d.Calculate(idx.testNames, nil, true)
		// If we have recalculated only a subset of tests, we want to keep the results from
		// the previous scans and overwrite what we have just recomputed.
		if len(idx.testNames) > 0 && len(idx.summaries[is]) > 0 {
			idx.summaries[is] = summary.MergeSorted(idx.summaries[is], sum)
		} else {
			idx.summaries[is] = sum
		}
	}
	return nil
}

// calcParamsetsInclude is the pipeline function to calculate the parameters from
// the full tile (not applying ignore rules)
func calcParamsetsInclude(_ context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	is := types.IncludeIgnoredTraces
	idx.paramsetSummaries[is] = paramsets.NewParamSummary(idx.cpxTile.GetTile(is), idx.dCounters[is])
	return nil
}

// calcParamsetsExclude is the pipeline function to calculate the parameters from
// the partial tile (applying ignore rules)
func calcParamsetsExclude(_ context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	is := types.ExcludeIgnoredTraces
	idx.paramsetSummaries[is] = paramsets.NewParamSummary(idx.cpxTile.GetTile(is), idx.dCounters[is])
	return nil
}

// calcBlame is the pipeline function to calculate the blame.
func calcBlame(ctx context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	exp, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrapf(err, "fetching expectations needed to calculate blame")
	}
	b, err := blame.New(idx.cpxTile.GetTile(types.ExcludeIgnoredTraces), exp)
	if err != nil {
		idx.blamer = nil
		return skerr.Wrapf(err, "calculating blame")
	}
	idx.blamer = b
	return nil
}

func writeKnownHashesList(ctx context.Context, state interface{}) error {
	idx := state.(*SearchIndex)

	// Only write the hash file if a storage client is available.
	if idx.gcsClient == nil {
		return nil
	}

	// Trigger writing the hashes list.
	go func() {
		// Make sure this doesn't hang indefinitely. 2 minutes was chosen as a time that's plenty
		// long to make sure it completes (usually takes only a few seconds).
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		byTest := idx.DigestCountsByTest(types.IncludeIgnoredTraces)
		// Collect all hashes in the tile.
		// TODO(kjlubick) Do we need to check that these images have actually been properly uploaded?
		//   For clients using goldctl, it doesn't matter since goldctl will re-upload images that
		//   aren't already in GCS. For clients that use known hashes to avoid writing more to disk
		//   than they need to (e.g. Skia), this may be important.
		hashes := types.DigestSet{}
		for _, test := range byTest {
			for k := range test {
				hashes[k] = true
			}
		}

		// Keep track of the number of known hashes since this directly affects how
		// many images the bots have to upload.
		metrics2.GetInt64Metric(knownHashesMetric).Update(int64(len(hashes)))
		if err := idx.gcsClient.WriteKnownDigests(ctx, hashes.Keys()); err != nil {
			sklog.Errorf("Error writing known digests list: %s", err)
		}
		sklog.Infof("Finished writing %d known hashes", len(hashes))
	}()
	return nil
}

// runWarmer is the pipeline function to run the warmer. It runs
// asynchronously since its results are not relevant for the searchIndex.
func runWarmer(ctx context.Context, state interface{}) error {
	idx := state.(*SearchIndex)

	is := types.IncludeIgnoredTraces
	exp, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrapf(err, "preparing to run warmer - expectations failure")
	}
	d := digesttools.NewClosestDiffFinder(exp, idx.dCounters[is], idx.diffStore)

	// We don't want to pass the whole digestCounters because the byTrace map actually takes
	// quite a lot of memory, with potentially millions of entries.
	wd := warmer.Data{
		TestSummaries: idx.summaries[is],
		DigestsByTest: idx.dCounters[is].ByTest(),
		SubsetOfTests: idx.testNames,
	}
	// Pass these in so as to allow the rest of the items in the index to be GC'd if needed.
	go func(warmer warmer.DiffWarmer, wd warmer.Data, d digesttools.ClosestDiffFinder) {
		// If there are somehow lots and lots of diffs or the warmer gets stuck, we should bail out
		// at some point to prevent amount of work being done on the diffstore (e.g. a remote
		// diffserver) from growing in an unbounded fashion.
		// 15 minutes was chosen based on the 90th percentile time looking at the metrics.
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()

		if err := warmer.PrecomputeDiffs(ctx, wd, d); err != nil {
			sklog.Warningf("Could not precompute diffs for %d summaries and %d test names: %s", len(wd.TestSummaries), len(wd.SubsetOfTests), err)
		}
	}(idx.warmer, wd, d)
	return nil
}

const (
	// maxAgeOfOpenCLsToIndex is the maximum time between now and a CL's last updated time that we
	// will still index.
	maxAgeOfOpenCLsToIndex = 20 * 24 * time.Hour
	// We only keep around open CLs in the index. When a CL is closed, we don't update the indices
	// any more. These entries will expire and be removed from the cache after
	// changelistCacheExpirationDuration time has passed.
	changelistCacheExpirationDuration = 10 * 24 * time.Hour
	// maxCLsToIndex is the maximum number of CLs we query each loop to index them. Hopefully this
	// limit isn't reached regularly.
	maxCLsToIndex = 2000
)

// calcChangelistIndices goes through all open changelists within a given window and computes
// an index of them (e.g. the untriaged digests).
func (ix *Indexer) calcChangelistIndices(ctx context.Context) {
	ctx, span := trace.StartSpan(ctx, "indexer_calcChangelistIndices")
	defer span.End()
	// Update the metric when we return (either from error or because we completed indexing).
	defer metrics2.GetInt64Metric(indexedCLsMetric).Update(int64(ix.changelistIndices.ItemCount()))
	// Make sure this doesn't take arbitrarily long.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	now := time.Now()
	primaryExp, err := ix.ExpectationsStore.Get(ctx)
	if err != nil {
		sklog.Errorf("Could not get expectations for changelist indices: %s", err)
		return
	}

	for _, system := range ix.ReviewSystems {
		// An arbitrary cut off to the amount of recent, open CLs we try to index.
		recent := now.Add(-maxAgeOfOpenCLsToIndex)
		xcl, _, err := system.Store.GetChangelists(ctx, clstore.SearchOptions{
			StartIdx:    0,
			Limit:       maxCLsToIndex,
			OpenCLsOnly: true,
			After:       recent,
		})
		if err != nil {
			sklog.Errorf("Could not get recent changelists: %s", err)
			return
		}

		sklog.Infof("Indexing %d CLs", len(xcl))

		const numChunks = 8 // arbitrarily picked, could likely be tuned based on contention of
		// changelistCache
		chunkSize := (len(xcl) / numChunks) + 1 // add one to avoid integer truncation.
		err = util.ChunkIterParallel(ctx, len(xcl), chunkSize, func(ctx context.Context, startIdx int, endIdx int) error {
			for _, cl := range xcl[startIdx:endIdx] {
				if err := ctx.Err(); err != nil {
					sklog.Errorf("Changelist indexing timed out (%v)", err)
					return nil
				}
				issueExpStore := ix.ExpectationsStore.ForChangelist(cl.SystemID, system.ID)
				clExps, err := issueExpStore.Get(ctx)
				if err != nil {
					return skerr.Wrapf(err, "loading expectations for cl %s (%s)", cl.SystemID, system.ID)
				}
				exps := expectations.Join(clExps, primaryExp)

				clKey := fmt.Sprintf("%s_%s", system.ID, cl.SystemID)
				clIdx, ok := ix.getCLIndex(clKey)
				// Ingestion should update this timestamp when it has uploaded a new file belonging to this
				// changelist. We add a bit of a buffer period to avoid potential issues with a file being
				// uploaded at the exact same time we create an index (skbug.com/10265).
				updatedWithGracePeriod := cl.Updated.Add(30 * time.Second)
				if !ok || clIdx.ComputedTS.Before(updatedWithGracePeriod) {
					ix.changelistsReindexed.Inc(1)
					// Compute it from scratch and store it to the index.
					xps, err := system.Store.GetPatchsets(ctx, cl.SystemID)
					if err != nil {
						return skerr.Wrap(err)
					}
					if len(xps) == 0 {
						continue
					}
					latestPS := xps[len(xps)-1]
					psID := tjstore.CombinedPSID{
						CL:  cl.SystemID,
						CRS: system.ID,
						PS:  latestPS.SystemID,
					}
					afterTime := time.Time{}
					var existingUntriagedResults []tjstore.TryJobResult
					// Test to see if we can do an incremental index (just for results that were uploaded
					// for this patchset since the last time we indexed).
					if ok && clIdx.LatestPatchset.PS == latestPS.SystemID {
						afterTime = clIdx.ComputedTS
						existingUntriagedResults = clIdx.UntriagedResults
					}
					xtjr, err := ix.TryJobStore.GetResults(ctx, psID, afterTime)
					if err != nil {
						return skerr.Wrap(err)
					}
					untriagedResults, params := indexTryJobResults(ctx, existingUntriagedResults, xtjr, exps)
					if err := ix.sendCLWorkToDiffCalculators(ctx, primaryExp, xtjr, system.ID+"_"+cl.SystemID); err != nil {
						return skerr.Wrap(err)
					}
					// Copy the existing ParamSet into the newly created one. It is important to copy it from
					// old into new (and not new into old), so we don't cause a race condition on the cached
					// ParamSet by writing to it while GetIndexForCL is reading from it.
					params.AddParamSet(clIdx.ParamSet)
					clIdx.ParamSet = params
					clIdx.LatestPatchset = psID
					clIdx.UntriagedResults = untriagedResults
					clIdx.ComputedTS = now
				}
				ix.changelistIndices.Set(clKey, &clIdx, ttlcache.DefaultExpiration)
			}
			return nil
		})
		if err != nil {
			sklog.Errorf("Error indexing changelists from CRS %s: %s", system.ID, err)
		}
	}
}

// indexTryJobResults goes through all the TryJobResults and returns results useful for indexing.
// Concretely, these results are a slice with just the untriaged results and a ParamSet with the
// observed params.
func indexTryJobResults(ctx context.Context, existing, newResults []tjstore.TryJobResult, exps expectations.Classifier) ([]tjstore.TryJobResult, paramtools.ParamSet) {
	ctx, span := trace.StartSpan(ctx, "indexer_indexTryJobResults")
	defer span.End()
	params := paramtools.ParamSet{}
	var newlyUntriagedResults []tjstore.TryJobResult
	for _, tjr := range newResults {
		params.AddParams(tjr.GroupParams)
		params.AddParams(tjr.ResultParams)
		params.AddParams(tjr.Options)
		tn := types.TestName(tjr.ResultParams[types.PrimaryKeyField])
		if exps.Classification(tn, tjr.Digest) == expectations.Untriaged {
			// If the same digest somehow shows up twice (maybe because of how we
			alreadyInList := false
			for _, existingResult := range existing {
				if existingResult.Digest == tjr.Digest && existingResult.ResultParams[types.PrimaryKeyField] == tjr.ResultParams[types.PrimaryKeyField] {
					alreadyInList = true
					break
				}
			}
			if !alreadyInList {
				newlyUntriagedResults = append(newlyUntriagedResults, tjr)
			}
		}
	}
	if len(newlyUntriagedResults) == 0 {
		return existing, params
	}

	if len(existing) == 0 {
		return newlyUntriagedResults, params
	}
	// make a copy of the slice, so as not to confuse the existing index.
	combined := make([]tjstore.TryJobResult, 0, len(existing)+len(newlyUntriagedResults))
	combined = append(combined, existing...)
	combined = append(combined, newlyUntriagedResults...)
	return combined, params
}

func (ix *Indexer) sendCLWorkToDiffCalculators(ctx context.Context, primaryExp expectations.Classifier, xtjr []tjstore.TryJobResult, clID string) error {
	ctx, span := trace.StartSpan(ctx, "indexer_sendCLWorkToDiffCalculators")
	defer span.End()
	if len(xtjr) == 0 {
		sklog.Infof("No new Tryjob results for CL %s", clID)
		return nil
	}
	idx := ix.getIndex()
	if idx == nil {
		// Should not happen because we compute the primary branch index synchronously
		// before starting to index CLs.
		return skerr.Fmt("Primary branch index not ready yet")
	}
	digestsPerGrouping, err := groupDataFromTryJobs(ctx, xtjr)
	if err != nil {
		return skerr.Wrap(err)
	}
	left, right, err := addDataFromPrimaryBranch(ctx, idx, digestsPerGrouping, primaryExp)
	if err != nil {
		return skerr.Wrap(err)
	}

	sklog.Infof("Sending diff messages for CL %s covering %d groupings to diffcalculator", clID, len(left))
	for hg := range left {
		grouping := paramtools.Params{
			types.CorpusField:     hg[0],
			types.PrimaryKeyField: hg[1],
		}
		leftDigests := left[hg].Keys()
		rightDigests := right[hg].Keys()
		// This should be pretty fast because it's just sending off the work, not blocking until
		// the work is calculated.
		if err := ix.DiffWorkPublisher.CalculateDiffs(ctx, grouping, leftDigests, rightDigests); err != nil {
			return skerr.Wrapf(err, "publishing diff calculation for CL %s - %d, %d digests in grouping %v", clID, len(leftDigests), len(rightDigests), grouping)
		}
	}
	return nil
}

// groupDataFromTryJobs takes the data from the provided TryJobResults and groups the unique
// digests by grouping.
func groupDataFromTryJobs(ctx context.Context, xtjr []tjstore.TryJobResult) (map[hashableGrouping]types.DigestSet, error) {
	ctx, span := trace.StartSpan(ctx, "groupDataFromTryJobs")
	span.AddAttributes(trace.Int64Attribute("data_points", int64(len(xtjr))))
	defer span.End()
	// The left and right digests will be the data from these tryjobs as well as the non-ignored
	// data on the primary branch for the corresponding groupings.
	digestsPerGrouping := map[hashableGrouping]types.DigestSet{}
	for _, tjr := range xtjr {
		if err := ctx.Err(); err != nil {
			return nil, skerr.Wrap(err)
		}
		traceKeys := paramtools.Params{}
		traceKeys.Add(tjr.GroupParams, tjr.ResultParams)
		grouping := getHashableGrouping(traceKeys)
		uniqueDigests := digestsPerGrouping[grouping]
		if len(uniqueDigests) == 0 {
			uniqueDigests = types.DigestSet{}
		}
		if tjr.Digest != tiling.MissingDigest {
			uniqueDigests[tjr.Digest] = true
		}
		digestsPerGrouping[grouping] = uniqueDigests
	}
	return digestsPerGrouping, nil
}

// addDataFromPrimaryBranch adds the triaged digests from the primary branch to
// the provided map and returns it as the first return value (the left digests). It adds those same
// digests to a new map and returns it as the second return value (the right digests).
func addDataFromPrimaryBranch(ctx context.Context, idx *SearchIndex, leftDigests map[hashableGrouping]types.DigestSet, exp expectations.Classifier) (map[hashableGrouping]types.DigestSet, map[hashableGrouping]types.DigestSet, error) {
	ctx, span := trace.StartSpan(ctx, "addDataFromPrimaryBranch")
	span.AddAttributes(trace.Int64Attribute("groupings", int64(len(leftDigests))))
	defer span.End()
	countByTest := idx.dCounters[types.IncludeIgnoredTraces].ByTest()
	rightDigests := make(map[hashableGrouping]types.DigestSet, len(leftDigests))
	// Add the digests from the primary branch (using the index)
	for grouping := range leftDigests {
		if err := ctx.Err(); err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		allLeftDigests := leftDigests[grouping]
		allRightDigests := types.DigestSet{}
		// We assume that test names are unique across corpora. This may not be true in general,
		// but it allows us to avoid iterating the tile again.
		for digest := range countByTest[types.TestName(grouping[1])] {
			if exp.Classification(types.TestName(grouping[1]), digest) != expectations.Untriaged {
				allLeftDigests[digest] = true
				allRightDigests[digest] = true
			}
		}
		// This won't make a new key in the map, so it should be safe overwrite this key and not
		// affect iteration.
		leftDigests[grouping] = allLeftDigests
		rightDigests[grouping] = allRightDigests
	}
	return leftDigests, rightDigests, nil
}

// getCLIndex is a helper that returns the appropriately typed element from changelistIndices.
// We return a struct and not a pointer so that we can update the index w/o having to need a mutex.
func (ix *Indexer) getCLIndex(key string) (ChangelistIndex, bool) {
	clIdx, ok := ix.changelistIndices.Get(key)
	if !ok || clIdx == nil {
		return ChangelistIndex{}, false
	}
	return *clIdx.(*ChangelistIndex), true
}

// GetIndexForCL implements the IndexSource interface.
func (ix *Indexer) GetIndexForCL(crs, clID string) *ChangelistIndex {
	key := fmt.Sprintf("%s_%s", crs, clID)
	clIdx, ok := ix.getCLIndex(key)
	if !ok {
		return nil
	}
	// Return a copy to prevent clients from messing with the cached version.
	return clIdx.Copy()
}

// sendWorkToDiffCalculators groups the digests seen per test and sends them to the DiffCalculators
// (via Pub/Sub). We cannot use the results of digestCounters since those do not report the corpus,
// which we need for groupings.
func (ix *Indexer) sendWorkToDiffCalculators(ctx context.Context, state interface{}) error {
	idx := state.(*SearchIndex)
	tile := idx.cpxTile.GetTile(types.IncludeIgnoredTraces)
	// For every digest on every trace within the sliding window (tile), compute the
	// unique digests for each grouping (i.e. test). These will be the left digests.
	leftDigestsPerGrouping := map[hashableGrouping]types.DigestSet{}
	for _, tr := range tile.Traces {
		grouping := getHashableGrouping(tr.Keys())
		uniqueDigests := leftDigestsPerGrouping[grouping]
		if len(uniqueDigests) == 0 {
			uniqueDigests = types.DigestSet{}
		}
		for _, d := range tr.Digests {
			if d != tiling.MissingDigest {
				uniqueDigests[d] = true
			}
		}
		leftDigestsPerGrouping[grouping] = uniqueDigests
	}

	exp, err := idx.expectationsStore.Get(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	// For every digest on every trace within the sliding window (tile), compute the
	// unique digests for each grouping (i.e. test). These will be the right digests, i.e.
	// all the "triaged" digests (including ignored, triaged digests).
	rightDigestsPerGrouping := map[hashableGrouping]types.DigestSet{}
	for _, tr := range tile.Traces {
		grouping := getHashableGrouping(tr.Keys())
		uniqueDigests := rightDigestsPerGrouping[grouping]
		if len(uniqueDigests) == 0 {
			uniqueDigests = types.DigestSet{}
		}
		for _, d := range tr.Digests {
			if d != tiling.MissingDigest && exp.Classification(tr.TestName(), d) != expectations.Untriaged {
				uniqueDigests[d] = true
			}
		}
		rightDigestsPerGrouping[grouping] = uniqueDigests
	}

	for hg, ds := range leftDigestsPerGrouping {
		grouping := paramtools.Params{
			types.CorpusField:     hg[0],
			types.PrimaryKeyField: hg[1],
		}
		leftDigests := ds.Keys()
		rightDigests := rightDigestsPerGrouping[hg].Keys()
		// This should be pretty fast because it's just sending off the work, not blocking until
		// the work is calculated.
		if err := ix.DiffWorkPublisher.CalculateDiffs(ctx, grouping, leftDigests, rightDigests); err != nil {
			return skerr.Wrapf(err, "publishing diff calculation of (%d, %d) digests in grouping %v", len(leftDigests), len(rightDigests), grouping)
		}
	}
	return nil
}

// hashableGrouping is the corpus and test name of a trace.
type hashableGrouping [2]string

func getHashableGrouping(keys paramtools.Params) [2]string {
	return [2]string{keys[types.CorpusField], keys[types.PrimaryKeyField]}
}

// Make sure SearchIndex fulfills the IndexSearcher interface
var _ IndexSearcher = (*SearchIndex)(nil)

// Make sure Indexer fulfills the IndexSource interface
var _ IndexSource = (*Indexer)(nil)
