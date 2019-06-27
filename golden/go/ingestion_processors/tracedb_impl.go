package ingestion_processors

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	tracedbServiceConfig = "TraceService"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD, newDeprecatedTraceDBProcessor)
}

// traceDBProcessor implements the ingestion.Processor interface for gold.
type traceDBProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

// implements the ingestion.Constructor signature.
func newDeprecatedTraceDBProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, _ *http.Client, _ eventbus.EventBus) (ingestion.Processor, error) {
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

// extractTraceDBEntries returns the traceDB entries to be inserted into the data store.
func extractTraceDBEntries(dm *dmResults) (map[tiling.TraceId]*tracedb.Entry, error) {
	ret := make(map[tiling.TraceId]*tracedb.Entry, len(dm.Results))
	for _, result := range dm.Results {
		traceId, params := idAndParams(dm, result)
		if ignoreResult(dm, params) {
			continue
		}

		ret[traceId] = &tracedb.Entry{
			Params: params,
			Value:  []byte(result.Digest),
		}
	}

	// If all results were ignored then we return an error.
	if len(ret) == 0 {
		return nil, fmt.Errorf("No valid results in file %s.", dm.name)
	}

	return ret, nil
}
