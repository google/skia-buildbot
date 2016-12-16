// Package indexer continously creates an index of the test results
// as the tiles, expectations and ignores change.
package indexer

import (
	"net/url"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/golden/go/blame"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/paramsets"
	"go.skia.org/infra/golden/go/pdag"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/tally"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/warmer"
)

const (
	// Event emitted when the indexer updates the search index.
	// Callback argument: *SearchIndex
	EV_INDEX_UPDATED = "indexer:index-updated"
)

// SearchIndex contains everything that is necessary to search
// our current knowledge about test results. It should be
// considered as immutable. Whenever the underlying data change
// a new index is calculated via a pdag.
type SearchIndex struct {
	tilePair        *types.TilePair
	tallies         *tally.Tallies
	summaries       *summary.Summaries
	paramsetSummary *paramsets.ParamSummary
	blamer          *blame.Blamer
	warmer          *warmer.Warmer

	// Used by the pdag pipeline.
	testNames []string
}

// newSearchIndex creates a new instance of SearchIndex. It is not intended to
// be used outside of this package. SearchIndex instances are created by the
// Indexer and retrieved via GetIndex().
func newSearchIndex(storages *storage.Storage, tilePair *types.TilePair) *SearchIndex {
	return &SearchIndex{
		tilePair:        tilePair,
		tallies:         tally.New(),
		summaries:       summary.New(storages),
		paramsetSummary: paramsets.New(),
		blamer:          blame.New(storages),
		warmer:          warmer.New(storages),
	}
}

// GetTile returns the current tile either with or without the ignored traces.
func (idx *SearchIndex) GetTile(includeIgnores bool) *tiling.Tile {
	if includeIgnores {
		return idx.tilePair.TileWithIgnores
	}
	return idx.tilePair.Tile
}

// Proxy to tally.Tallies.ByTest
func (idx *SearchIndex) TalliesByTest() map[string]tally.Tally {
	return idx.tallies.ByTest()
}

// Proxy to tally.Tallies.ByTrace
func (idx *SearchIndex) TalliesByTrace() map[string]tally.Tally {
	return idx.tallies.ByTrace()
}

// ByQuery returns a Tally of all the digests that match the given query.
func (idx *SearchIndex) TalliesByQuery(query url.Values, includeIgnores bool) tally.Tally {
	return idx.tallies.ByQuery(idx.GetTile(includeIgnores), query)
}

// Proxy to summary.Summary.Get.
func (idx *SearchIndex) GetSummaries() map[string]*summary.Summary {
	return idx.summaries.Get()
}

// Proxy to summary.CalcSummaries.
func (idx *SearchIndex) CalcSummaries(testNames []string, query url.Values, includeIgnores, head bool) (map[string]*summary.Summary, error) {
	if includeIgnores {
		return idx.summaries.CalcSummaries(idx.tilePair.TileWithIgnores, testNames, query, head)
	}
	return idx.summaries.CalcSummaries(idx.tilePair.Tile, testNames, query, head)
}

// Proxy to paramsets.Get
func (idx *SearchIndex) GetParamsetSummary(test, digest string, includeIgnores bool) map[string][]string {
	return idx.paramsetSummary.Get(test, digest, includeIgnores)
}

// Proxy to blame.Blamer.GetBlame.
func (idx *SearchIndex) GetBlame(test, digest string, commits []*tiling.Commit) *blame.BlameDistribution {
	return idx.blamer.GetBlame(test, digest, commits)
}

// Indexer is the type that drive continously indexing as the underlying
// data change. It uses a DAG that encodes the dependencies of the
// different components of an index and creates a processing pipeline on top
// of it.
type Indexer struct {
	storages   *storage.Storage
	pipeline   *pdag.Node
	blamerNode *pdag.Node
	lastIndex  *SearchIndex
	testNames  []string
	mutex      sync.RWMutex
}

// New returns a new Indexer instance. It synchronously indexes the initiallly
// available tile. If the indexing fails an error is returned.
// The provided interval defines how often the index should be refreshed.
func New(storages *storage.Storage, interval time.Duration) (*Indexer, error) {
	ret := &Indexer{
		storages: storages,
	}

	// Set up the processing pipeline.
	root := pdag.NewNode(pdag.NoOp)

	// Add the blamer and tallies
	blamerNode := root.Child(calcBlame)
	tallyNode := root.Child(calcTallies)

	// parameters depend on tallies.
	tallyNode.Child(calcParamsets)

	// summaries depend on tallies and blamer.
	summaryNode := pdag.NewNode(calcSummaries, tallyNode, blamerNode)

	// The warmer depends on tallies and summaries.
	pdag.NewNode(runWarmer, summaryNode, tallyNode)

	// Set the result on the Indexer instance.
	pdag.NewNode(ret.setIndex, summaryNode)

	ret.pipeline = root
	ret.blamerNode = blamerNode

	// Process the first tile and start the indexing process.
	return ret, ret.start(interval)
}

// GetIndex returns the current index, which is updated continously in the
// background. The returned instances of SearchIndex can be considered immutable
// and is not going to change. It should be used to handle an entire request
// to provide consistent information.
func (ixr *Indexer) GetIndex() *SearchIndex {
	ixr.mutex.RLock()
	defer ixr.mutex.RUnlock()
	return ixr.lastIndex
}

// start builds the initial index and starts the background
// process to continously build indices.
func (ixr *Indexer) start(interval time.Duration) error {
	// Build the first index synchronously.
	tileStream := ixr.storages.GetTileStreamNow(interval)
	if err := ixr.indexTilePair(<-tileStream); err != nil {
		return err
	}

	// When the expecations change, update the blamer and its dependents.
	expCh := make(chan []string)
	ixr.storages.EventBus.SubscribeAsync(expstorage.EV_EXPSTORAGE_CHANGED, func(e interface{}) {
		// Schedule the list of test names to be recalculated.
		expCh <- e.([]string)
	})

	// Keep building indices as tiles become available and expecations change.
	go func() {
		var tilePair *types.TilePair
		for {
			// See if there is a tile.
			tilePair = nil
			select {
			case tilePair = <-tileStream:
			default:
			}

			// Drain all the tests that might have changed.
			var testNames []string = nil
			done := false
			for !done {
				select {
				case tn := <-expCh:
					testNames = append(testNames, tn...)
				default:
					done = true
				}
			}

			// If there is tile, re-index everything and forget the
			// individual tests that changed.
			if tilePair != nil {
				if err := ixr.indexTilePair(tilePair); err != nil {
					sklog.Errorf("Unable to index tile: %s", err)
				}
			} else if len(testNames) > 0 {
				ixr.indexTests(testNames)
			}
		}
	}()

	return nil
}

// indexTilePair runs the given TilePair through the the indexing pipeline.
func (ixr *Indexer) indexTilePair(tilePair *types.TilePair) error {
	defer timer.New("indexTilePair").Stop()
	// Create a new index from the given tile.
	return ixr.pipeline.Trigger(newSearchIndex(ixr.storages, tilePair))
}

// indexTest creates an updated index by indexing the given list of tests.
func (ixr *Indexer) indexTests(testNames []string) {
	defer timer.New("indexTests").Stop()
	lastIdx := ixr.GetIndex()
	newIdx := &SearchIndex{
		tilePair:        lastIdx.tilePair,
		tallies:         lastIdx.tallies,
		summaries:       lastIdx.summaries.Clone(),
		paramsetSummary: lastIdx.paramsetSummary,
		blamer:          blame.New(ixr.storages),
		warmer:          warmer.New(ixr.storages),
		testNames:       testNames,
	}

	if err := ixr.blamerNode.Trigger(newIdx); err != nil {
		sklog.Errorf("Error indexing tests: %v \n\n Got error: %s", testNames, err)
	}
}

// setIndex sets the lastIndex value at the very end of the pipeline.
func (ixr *Indexer) setIndex(state interface{}) error {
	newIndex := state.(*SearchIndex)
	ixr.mutex.Lock()
	defer ixr.mutex.Unlock()
	ixr.lastIndex = newIndex
	if ixr.storages.EventBus != nil {
		ixr.storages.EventBus.Publish(EV_INDEX_UPDATED, state)
	}
	return nil
}

// calcTallies is the pipeline function to calculate the tallies.
func calcTallies(state interface{}) error {
	idx := state.(*SearchIndex)
	idx.tallies.Calculate(idx.tilePair.TileWithIgnores)
	return nil
}

// calcSummaries is the pipeline function to calculate the summaries.
func calcSummaries(state interface{}) error {
	idx := state.(*SearchIndex)
	err := idx.summaries.Calculate(idx.tilePair.Tile, idx.testNames, idx.tallies, idx.blamer)
	return err
}

// calcParamsets is the pipeline function to calculate the parameters.
func calcParamsets(state interface{}) error {
	idx := state.(*SearchIndex)
	idx.paramsetSummary.Calculate(idx.tilePair, idx.tallies)
	return nil
}

// calcBlame is the pipeline function to calculate the blame.
func calcBlame(state interface{}) error {
	idx := state.(*SearchIndex)
	err := idx.blamer.Calculate(idx.tilePair.Tile)
	return err
}

// runWamer is the pipeline function to run the wamer. It runs it
// asynchronously since its results are not relevant for the searchIndex.
func runWarmer(state interface{}) error {
	idx := state.(*SearchIndex)
	go idx.warmer.Run(idx.tilePair.TileWithIgnores, idx.summaries, idx.tallies)
	return nil
}
