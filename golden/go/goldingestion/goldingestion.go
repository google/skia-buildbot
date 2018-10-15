package goldingestion

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Configuration option that identifies the address of the traceDB service.
	CONFIG_TRACESERVICE = "TraceService"

	// Configuration option for the secondary repository.
	CONFIG_SECONDARY_REPO = "SecondaryRepoURL"

	// Configuration option to define the regular expression to extract the
	// commit from the secondary repo. The provided regular expression must
	// contain exactly one group which maps to the commit in the DEPS file.
	CONFIG_SECONDARY_REG_EX = "SecondaryRegEx"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD, newGoldProcessor)
}

// goldProcessor implements the ingestion.Processor interface for gold.
type goldProcessor struct {
	traceDB tracedb.DB
	vcs     vcsinfo.VCS
}

type extractIDFn func(*vcsinfo.LongCommit, *DMResults) (*tracedb.CommitID, error)

// implements the ingestion.Constructor signature.
func newGoldProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (ingestion.Processor, error) {
	traceDB, err := tracedb.NewTraceServiceDBFromAddress(config.ExtraParams[CONFIG_TRACESERVICE], types.GoldenTraceBuilder)
	if err != nil {
		return nil, err
	}

	ret := &goldProcessor{
		traceDB: traceDB,
		vcs:     vcs,
	}
	return ret, nil
}

// See ingestion.Processor interface.
func (g *goldProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	dmResults, err := processDMResults(resultsFile)
	if err != nil {
		return err
	}

	var commit *vcsinfo.LongCommit = nil
	// If the target commit is not in the primary repository we look it up
	// in the secondary that has the primary as a dependency.
	targetHash, err := g.getCanonicalCommitHash(ctx, dmResults.GitHash)
	if err != nil {
		return err
	}

	commit, err = g.vcs.Details(ctx, targetHash, true)
	if err != nil {
		return err
	}

	if !commit.Branches["master"] {
		sklog.Warningf("Commit %s is not in master branch. Got branches: %v", commit.Hash, commit.Branches)
		return ingestion.IgnoreResultsFileErr
	}

	// Add the column to the trace db.
	cid, err := g.getCommitID(commit, dmResults)
	if err != nil {
		return err
	}

	// Get the entries that should be added to the tracedb.
	entries, err := extractTraceDBEntries(dmResults)
	if err != nil {
		return err
	}

	// Write the result to the tracedb.
	err = g.traceDB.Add(cid, entries)
	return err
}

// See ingestion.Processor interface.
func (g *goldProcessor) BatchFinished() error { return nil }

// getCommitID extracts the commitID from the given commit and dm results.
func (g *goldProcessor) getCommitID(commit *vcsinfo.LongCommit, dmResults *DMResults) (*tracedb.CommitID, error) {
	return &tracedb.CommitID{
		Timestamp: commit.Timestamp.Unix(),
		ID:        commit.Hash,
		Source:    "master",
	}, nil
}

// getCanonicalCommitHash returns the commit hash in the primary repository. If the given
// target hash is not in the primary repository it will try and find it in the secondary
// repository which has the primary as a dependency.
func (g *goldProcessor) getCanonicalCommitHash(ctx context.Context, targetHash string) (string, error) {
	// If it is not in the primary repo.
	if !isCommit(ctx, g.vcs, targetHash) {
		// Extract the commit.
		foundCommit, err := g.vcs.ResolveCommit(ctx, targetHash)
		if err != nil {
			return "", fmt.Errorf("Unable to resolve commit %s in secondary repo. Got err: %s", targetHash, err)
		}

		if foundCommit == "" {
			return "", fmt.Errorf("Unable to resolve commit %s in secondary repo.", targetHash)
		}

		// Check if the found commit is actually in the primary repository.
		if !isCommit(ctx, g.vcs, foundCommit) {
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
