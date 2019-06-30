package ingestion_processors

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
)

const (
	// Configuration option that identifies a tracestore backed by BigTable.
	btGoldIngester = "gold-bt"

	btProjectConfig  = "BTProjectID"
	btInstanceConfig = "BTInstance"
	btTableConfig    = "BTTable"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(btGoldIngester, newBTTraceStoreProcessor)
}

// newTraceStoreProcessor implements the ingestion.Constructor signature and creates
// a Processor that uses a BigTable-backed tracestore.
func newBTTraceStoreProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client, _ eventbus.EventBus) (ingestion.Processor, error) {
	btc := bt_tracestore.BTConfig{
		ProjectID:  config.ExtraParams[btProjectConfig],
		InstanceID: config.ExtraParams[btInstanceConfig],
		TableID:    config.ExtraParams[btTableConfig],
		VCS:        vcs,
	}

	bts, err := bt_tracestore.New(context.Background(), btc, true)
	if err != nil {
		return nil, skerr.Fmt("could not instantiate BT tracestore: %s", err)
	}
	return &btProcessor{
		ts:  bts,
		vcs: btc.VCS,
	}, nil
}

// btProcessor implements the ingestion.Processor interface for gold using
// the BigTable TraceStore
type btProcessor struct {
	ts  tracestore.TraceStore
	vcs vcsinfo.VCS
}

// Process implements the ingestion.Processor interface.
func (b *btProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	dmResults, err := processDMResults(resultsFile)
	if err != nil {
		return skerr.Fmt("could not process results file: %s", err)
	}

	if len(dmResults.Results) == 0 {
		sklog.Infof("ignoring file %s because it has no results", resultsFile.Name())
		return ingestion.IgnoreResultsFileErr
	}

	var commit *vcsinfo.LongCommit = nil
	// If the target commit is not in the primary repository we look it up
	// in the secondary that has the primary as a dependency.
	targetHash, err := getCanonicalCommitHash(ctx, b.vcs, dmResults.GitHash)
	if err != nil {
		if err == ingestion.IgnoreResultsFileErr {
			return ingestion.IgnoreResultsFileErr
		}
		return skerr.Fmt("could not identify canonical commit from %q: %s", dmResults.GitHash, err)
	}

	commit, err = b.vcs.Details(ctx, targetHash, true)
	if err != nil {
		return skerr.Fmt("could not get details for git commit %q: %s", targetHash, err)
	}

	if !commit.Branches["master"] {
		sklog.Warningf("Commit %s is not in master branch. Got branches: %v", commit.Hash, commit.Branches)
		return ingestion.IgnoreResultsFileErr
	}

	// Get the entries that should be added to the tracestore.
	entries, err := extractTraceStoreEntries(dmResults)
	if err != nil {
		return skerr.Fmt("could not create entries for results: %s", err)
	}

	t := time.Unix(resultsFile.TimeStamp(), 0)

	defer shared.NewMetricsTimer("put_tracestore_entry").Stop()

	sklog.Infof("tracestore.Put(%s, %d entries, %s)", targetHash, len(entries), t)
	// Write the result to the tracestore.
	err = b.ts.Put(ctx, targetHash, entries, t)
	if err != nil {
		return skerr.Fmt("could not add to tracedb: %s", err)
	}
	return nil
}

// Process implements the ingestion.Processor interface.
func (b *btProcessor) BatchFinished() error { return nil }

func extractTraceStoreEntries(dm *dmResults) ([]*tracestore.Entry, error) {
	ret := make([]*tracestore.Entry, 0, len(dm.Results))
	for _, result := range dm.Results {
		_, params := idAndParams(dm, result)
		if ignoreResult(dm, params) {
			continue
		}

		ret = append(ret, &tracestore.Entry{
			Params: params,
			Digest: result.Digest,
		})
	}

	// If all results were ignored then we return an error.
	if len(ret) == 0 {
		return nil, fmt.Errorf("No valid results in file %s.", dm.name)
	}

	return ret, nil
}
