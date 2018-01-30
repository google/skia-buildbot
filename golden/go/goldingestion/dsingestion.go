package goldingestion

import (
	"context"
	"fmt"
	"net/http"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/dstilestore"
)

// Register the processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_DS_GOLD, newDSGoldProcessor)
}

// goldDSProcessor implements the ingestion.Processor interface for gold.
type goldDSProcessor struct {
	vcs                vcsinfo.VCS
	ctx                context.Context
	client             *dstilestore.DSTileStore
	numDigestsIngested metrics2.Counter
}

// implements the ingestion.Constructor signature.
func newDSGoldProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	namespace, ok := config.ExtraParams["Namespace"]
	if !ok {
		return nil, fmt.Errorf("Namespace is a required ExtraParams in ingestion config.")
	}
	project, ok := config.ExtraParams["Project"]
	if !ok {
		return nil, fmt.Errorf("Project is a required ExtraParams in ingestion config.")
	}
	ds.Init(project, namespace)

	ret := &goldDSProcessor{
		vcs:                vcs,
		ctx:                context.Background(),
		client:             dstilestore.NewDSTileStore(context.Background(), ds.DS),
		numDigestsIngested: metrics2.GetCounter("num_digests_ingested", nil),
	}
	return ret, nil
}

// See ingestion.Processor interface.
func (g *goldDSProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
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
	cindex, err := g.vcs.IndexOf(g.ctx, commit.Hash)
	if err != nil {
		return err
	}

	// Get the entries that should be added to the datastore.
	entries, err := dmResults.getEntries()
	if err != nil {
		return err
	}

	// Write the result to the datastore.
	err = g.client.Add(cindex, entries)
	g.numDigestsIngested.Inc(int64(len(entries)))
	return err
}

// See ingestion.Processor interface.
func (g *goldDSProcessor) BatchFinished() error { return nil }

// getCanonicalCommitHash returns the commit hash in the primary repository. If the given
// target hash is not in the primary repository it will try and find it in the secondary
// repository which has the primary as a dependency.
func (g *goldDSProcessor) getCanonicalCommitHash(ctx context.Context, targetHash string) (string, error) {
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
