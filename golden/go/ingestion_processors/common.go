package ingestion_processors

import (
	"context"
	"io"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
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
func processGoldResults(rf ingestion.ResultFileLocation) (*jsonio.GoldResults, error) {
	defer shared.NewMetricsTimer("read_dm_results").Stop()
	r, err := rf.Open()
	if err != nil {
		return nil, skerr.Wrapf(err, "opening ResultFileLocation %s", rf.Name())
	}

	gr, err := parseGoldResultsFromReader(r)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing ResultFileLocation %s", rf.Name())
	}
	return gr, nil
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
