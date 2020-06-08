package ingestion_processors

import (
	"context"
	"io"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
)

// parseGoldResultsFromReader parses the JSON stream out of the io.ReadCloser
// into a jsonio.GoldResults instance and closes the reader.
func parseGoldResultsFromReader(r io.ReadCloser) (*jsonio.GoldResults, error) {
	defer util.Close(r)

	gr, err := jsonio.ParseGoldResults(r)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return gr, nil
}

// processGoldResults opens the given JSON input file and processes it, converting
// it into a jsonio.GoldResults object and returning it.
func processGoldResults(ctx context.Context, rf ingestion.ResultFileLocation) (*jsonio.GoldResults, error) {
	defer shared.NewMetricsTimer("read_dm_results").Stop()
	r, err := rf.Open(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "opening ResultFileLocation %s", rf.Name())
	}

	gr, err := parseGoldResultsFromReader(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing ResultFileLocation %s", rf.Name())
	}
	return gr, nil
}

// getCanonicalCommitHash returns the commit hash in the primary repository. If the given
// target hash is not in the primary repository it will try and find it in the secondary
// repository which has the primary as a dependency.
func getCanonicalCommitHash(ctx context.Context, vcs vcsinfo.VCS, targetHash string) (string, error) {
	if isCommit(ctx, vcs, targetHash) {
		return targetHash, nil
	}
	// TODO(kjlubick) We need a way to handle secondary repos (probably not something that
	//   clutters the VCS interface). skbug.com/9628
	sklog.Warningf("Unable to find commit %s in primary repo and no secondary configured", targetHash)

	c := vcs.LastNIndex(3)
	if len(c) == 3 {
		sklog.Debugf("Last three commits were %s on %s, %s on %s, and %s on %s",
			c[0].Hash, c[0].Timestamp, c[1].Hash, c[1].Timestamp, c[2].Hash, c[2].Timestamp)
	} else {
		sklog.Debugf("Last commits: %v", c)
	}
	return "", ingestion.IgnoreResultsFileErr
}

// isCommit returns true if the given commit is in vcs.
func isCommit(ctx context.Context, vcs vcsinfo.VCS, commitHash string) bool {
	ret, err := vcs.Details(ctx, commitHash, false)
	if err != nil {
		// Let's try updating the VCS - perhaps this is a new commit?
		if err := vcs.Update(ctx, true, false); err != nil {
			sklog.Errorf("Could not update VCS: %s", err)
			return false
		}
		ret, err = vcs.Details(ctx, commitHash, false)
	}
	return err == nil && ret != nil
}
