package goldingestion

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
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tracestore"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	tracedbServiceConfig = "TraceService"

	btGoldIngester = "gold-bt"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD, newTraceDBProcessor)
	ingestion.Register(btGoldIngester, newTraceStoreProcessor)
}

// traceDBProcessor implements the ingestion.Processor interface for gold.
type traceDBProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// implements the ingestion.Constructor signature.
func newTraceDBProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client, _ eventbus.EventBus) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[tracedbServiceConfig], types.GoldenTraceBuilder)
	if err != nil {
		return nil, err
	}

	ret := &traceDBProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}
	return ret, nil
}

// See ingestion.Processor interface.
func (g *traceDBProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
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
	targetHash, err := getCanonicalCommitHash(ctx, g.vcs, dmResults.GitHash)
	if err != nil {
		if err == ingestion.IgnoreResultsFileErr {
			return ingestion.IgnoreResultsFileErr
		}
		return skerr.Fmt("could not identify canonical commit from %q: %s", dmResults.GitHash, err)
	}

	commit, err = g.vcs.Details(ctx, targetHash, true)
	if err != nil {
		return skerr.Fmt("could not get details for git commit %q: %s", targetHash, err)
	}

	if !commit.Branches["master"] {
		sklog.Warningf("Commit %s is not in master branch. Got branches: %v", commit.Hash, commit.Branches)
		return ingestion.IgnoreResultsFileErr
	}

	// Add the column to the trace db.
	cid, err := g.getCommitID(commit)
	if err != nil {
		return skerr.Fmt("could not get trace db id: %s", err)
	}

	// Get the entries that should be added to the tracedb.
	entries, err := extractTraceDBEntries(dmResults)
	if err != nil {
		return skerr.Fmt("could not create entries for results: %s", err)
	}

	// Write the result to the tracedb.
	err = g.traceDB.Add(cid, entries)
	if err != nil {
		return skerr.Fmt("could not add to tracedb: %s", err)
	}
	return nil
}

// See ingestion.Processor interface.
func (g *traceDBProcessor) BatchFinished() error { return nil }

// getCommitID extracts the commitID from the given commit.
func (g *traceDBProcessor) getCommitID(commit *vcsinfo.LongCommit) (*tracedb.CommitID, error) {
	return &tracedb.CommitID{
		Timestamp: commit.Timestamp.Unix(),
		ID:        commit.Hash,
		Source:    "master",
	}, nil
}

// getCanonicalCommitHash returns the commit hash in the primary repository. If the given
// target hash is not in the primary repository it will try and find it in the secondary
// repository which has the primary as a dependency.
func getCanonicalCommitHash(ctx context.Context, vcs vcsinfo.VCS, targetHash string) (string, error) {
	// If it is not in the primary repo.
	if !isCommit(ctx, vcs, targetHash) {
		// Extract the commit.
		foundCommit, err := vcs.ResolveCommit(ctx, targetHash)
		if err != nil && err != vcsinfo.NoSecondaryRepo {
			return "", fmt.Errorf("Unable to resolve commit %s in primary or secondary repo. Got err: %s", targetHash, err)
		}

		if foundCommit == "" {
			if err == vcsinfo.NoSecondaryRepo {
				sklog.Warningf("Unable to find commit %s in primary or secondary repo.", targetHash)
			} else {
				sklog.Warningf("Unable to find commit %s in primary repo and no secondary configured.", targetHash)
			}
			return "", ingestion.IgnoreResultsFileErr
		}

		// Check if the found commit is actually in the primary repository. This could indicate misconfiguration
		// of the secondary repo.
		if !isCommit(ctx, vcs, foundCommit) {
			return "", fmt.Errorf("Found invalid commit %s in secondary repo at commit %s. Not contained in primary repo.", foundCommit, targetHash)
		}
		sklog.Infof("Commit translation: %s -> %s", targetHash, foundCommit)
		targetHash = foundCommit
	}
	return targetHash, nil
}

// isCommit returns true if the given commit is in vcs.
func isCommit(ctx context.Context, vcs vcsinfo.VCS, commitHash string) bool {
	ret, err := vcs.Details(ctx, commitHash, false)
	return (err == nil) && (ret != nil)
}

const (
	btProjectConfig  = "BTProjectID"
	btInstanceConfig = "BTInstance"
	btTableConfig    = "BTTable"
)

func newTraceStoreProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client, _ eventbus.EventBus) (ingestion.Processor, error) {
	btc := bt_tracestore.BTConfig{
		ProjectID:  config.ExtraParams[btProjectConfig],
		InstanceID: config.ExtraParams[btInstanceConfig],
		TableID:    config.ExtraParams[btTableConfig],
		VCS:        vcs,
	}

	return NewTraceStoreProcessor(btc)
}

func NewTraceStoreProcessor(btc bt_tracestore.BTConfig) (ingestion.Processor, error) {
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

func extractTraceStoreEntries(dm *DMResults) ([]*tracestore.Entry, error) {
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
