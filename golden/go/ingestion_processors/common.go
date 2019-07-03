package ingestion_processors

import (
	"context"
	"fmt"
	"io"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
)

// dmResults enhances GoldResults with fields used for internal processing.
type dmResults struct {
	*jsonio.GoldResults

	// name is the name/path of the file where this came from.
	name string
}

// Name returns the name/path from which these results were parsed.
func (d *dmResults) Name() string {
	return d.name
}

// parseDMResultsFromReader parses the JSON stream out of the io.ReadCloser
// into a DMResults instance and closes the reader.
func parseDMResultsFromReader(r io.ReadCloser, name string) (*dmResults, error) {
	defer util.Close(r)

	goldResults, _, err := jsonio.ParseGoldResults(r)
	if err != nil {
		return nil, skerr.Fmt("Failed to decode JSON: %s", err)
	}

	dmResults := &dmResults{GoldResults: goldResults}
	dmResults.name = name
	return dmResults, nil
}

// processDMResults opens the given JSON input file and processes it, converting
// it into a goldingestion.dmResults object and returning it.
func processDMResults(rf ingestion.ResultFileLocation) (*dmResults, error) {
	defer shared.NewMetricsTimer("read_dm_results").Stop()
	r, err := rf.Open()
	if err != nil {
		return nil, skerr.Fmt("could not open file %s: %s", rf.Name(), err)
	}

	return parseDMResultsFromReader(r, rf.Name())
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
