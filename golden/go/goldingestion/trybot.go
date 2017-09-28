package goldingestion

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/depot_tools"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
)

const (
	CONFIG_RIETVELD_CODE_REVIEW_URL = "RietveldCodeReviewURL"
	CONFIG_GERRIT_CODE_REVIEW_URL   = "GerritCodeReviewURL"
	TIMESTAMP_LRU_CACHE_SIZE        = 1000
)

func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYBOT, newGoldTrybotProcessor)
}

type goldTrybotProcessor struct {
	*goldProcessor
	rietveldReview *rietveld.Rietveld
	gerritReview   *gerrit.Gerrit

	//    map[issue:patchset] -> timestamp.
	cache      map[string]time.Time
	cacheMutex sync.Mutex
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, secondaryVCS vcsinfo.VCS, ex depot_tools.DEPSExtractor) (ingestion.Processor, error) {
	processor, err := newGoldProcessor(vcs, config, client, secondaryVCS, ex)
	if err != nil {
		return nil, err
	}

	ingestionStore, err := NewIngestionStore(config.ExtraParams[CONFIG_TRACESERVICE])
	if err != nil {
		return nil, fmt.Errorf("Unable to open ingestion store: %s", err)
	}

	// Get the underlying goldProcessor.
	gProcessor := processor.(*goldProcessor)
	gProcessor.ingestionStore = ingestionStore

	gerritURL := config.ExtraParams[CONFIG_GERRIT_CODE_REVIEW_URL]
	rietveldURL := config.ExtraParams[CONFIG_RIETVELD_CODE_REVIEW_URL]
	if (gerritURL == "") || (rietveldURL == "") {
		return nil, fmt.Errorf("Missing URLs for rietveld and/or gerrit code review systems. Got values: ('%s', '%s')", rietveldURL, gerritURL)
	}

	gerritReview, err := gerrit.NewGerrit(gerritURL, "", nil)
	if err != nil {
		return nil, err
	}

	ret := &goldTrybotProcessor{
		goldProcessor:  gProcessor,
		rietveldReview: rietveld.New(rietveldURL, nil),
		gerritReview:   gerritReview,
		cache:          map[string]time.Time{},
	}

	// Change the function to extract the commitID.
	gProcessor.extractID = ret.getCommitID
	return ret, nil
}

// getCommitID overrides the function with the same name in goldProcessor.
func (g *goldTrybotProcessor) getCommitID(commit *vcsinfo.LongCommit, dmResults *DMResults) (*tracedb.CommitID, error) {
	var ts time.Time
	var ok bool
	var err error
	var cacheId = fmt.Sprintf("%d:%d", dmResults.Issue, dmResults.Patchset)

	err = func() error {
		g.cacheMutex.Lock()
		defer g.cacheMutex.Unlock()
		if ts, ok = g.cache[cacheId]; !ok {
			if ts, err = g.getCreatedTimeStamp(dmResults); err != nil {
				return err
			}

			// p.cache is a very crude LRU cache.
			if len(g.cache) > TIMESTAMP_LRU_CACHE_SIZE {
				g.cache = map[string]time.Time{}
			}
			g.cache[cacheId] = ts
		}
		return nil
	}()
	if err != nil {
		return nil, err
	}

	// Get the source (url) from the issue.
	var source string
	if dmResults.isGerritIssue() {
		source = g.gerritReview.Url(dmResults.Issue)
	} else {
		source = g.rietveldReview.Url(dmResults.Issue)
	}
	return &tracedb.CommitID{
		Timestamp: ts.Unix(),
		ID:        strconv.FormatInt(dmResults.Patchset, 10),
		Source:    source,
	}, nil
}

// getCreatedTimeStamp returns the timestamp of the patchset contained in
// DMResult either from Gerrit or Rietveld.
func (g *goldTrybotProcessor) getCreatedTimeStamp(dmResults *DMResults) (time.Time, error) {
	if dmResults.isGerritIssue() {
		issueProps, err := g.gerritReview.GetIssueProperties(dmResults.Issue)
		if err != nil {
			return time.Time{}, err
		}

		if len(issueProps.Patchsets) < int(dmResults.Patchset) {
			return time.Time{}, fmt.Errorf("Given patchset (%d) number is not available from Gerrit. Only found %d patchsets.", dmResults.Patchset, len(issueProps.Patchsets))
		}

		return issueProps.Patchsets[dmResults.Patchset-1].Created, nil
	} else {
		patchinfo, err := g.getRietveldPatchset(dmResults.Issue, dmResults.Patchset, dmResults.Name())
		if err != nil {
			return time.Time{}, err
		}
		return patchinfo.Created, nil
	}
}

// getPatchset retrieves the patchset. If it does not exist (but the Rietveld issue exists)
// it will return a ingestion.IgnoreResultsFileErr indicating that this input file should be ignored.
func (g *goldTrybotProcessor) getRietveldPatchset(issueID int64, patchsetID int64, name string) (*rietveld.Patchset, error) {
	// TODO(stephana): This should be a side effect of Rietveld to Gerrit transition.
	// Remove once transition complete. In the meantime log a warning for ingestigation.
	if issueID == 0 {
		sklog.Warningf("Received issue number 0 in file %s", name)
		return nil, ingestion.IgnoreResultsFileErr
	}

	patchinfo, err := g.rietveldReview.GetPatchset(issueID, patchsetID)
	if err == nil {
		return patchinfo, nil
	}

	// If we can find the issue, check if the patchset has been removed.
	var issueInfo *rietveld.Issue
	if issueInfo, err = g.rietveldReview.GetIssueProperties(issueID, false); err == nil {
		found := false
		for _, pset := range issueInfo.Patchsets {
			if pset == patchsetID {
				found = true
				break
			}
		}

		// This patchset is no longer available ignore the result file.
		if !found {
			sklog.Warningf("Rietveld issue/patchset (%d/%d) does not exist.", issueID, patchsetID)
			return nil, ingestion.IgnoreResultsFileErr
		}
		// We should not reach this point. Investigate manually if we do.
		return nil, fmt.Errorf("Found patchset %d for issue %d, but unable to retrieve it.", patchsetID, issueID)
	}

	return nil, fmt.Errorf("Failed to retrieve trybot issue and patch info for (%d, %d). Got Error: %s", issueID, patchsetID, err)
}

// ExtractIssueInfo returns the issue id and the patchset id for a given commitID.
func ExtractIssueInfo(commitID *tracedb.CommitID, rietveldReview *rietveld.Rietveld, gerritReview *gerrit.Gerrit) (string, string) {
	issue, ok := gerritReview.ExtractIssue(commitID.Source)
	if ok {
		return issue, commitID.ID
	}
	issue, _ = rietveldReview.ExtractIssue(commitID.Source)
	return issue, commitID.ID
}
