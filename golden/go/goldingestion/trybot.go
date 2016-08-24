package goldingestion

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/rietveld"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
)

const (
	CONFIG_CODE_REVIEW_URL   = "CodeReviewURL"
	TIMESTAMP_LRU_CACHE_SIZE = 1000
)

func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYBOT, newGoldTrybotProcessor)
}

type goldTrybotProcessor struct {
	*goldProcessor
	review *rietveld.Rietveld

	//    map[issue:patchset] -> timestamp.
	cache map[string]time.Time
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	processor, err := newGoldProcessor(vcs, config, client)
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
	ret := &goldTrybotProcessor{
		goldProcessor: gProcessor,
		review:        rietveld.New(config.ExtraParams[CONFIG_CODE_REVIEW_URL], nil),
		cache:         map[string]time.Time{},
	}

	// Change the function to extract the commitID.
	gProcessor.extractID = ret.getCommitID
	return ret, nil
}

// getCommitID overrides the function with the same name in goldProcessor.
func (g *goldTrybotProcessor) getCommitID(commit *vcsinfo.LongCommit, dmResults *DMResults) (*tracedb.CommitID, error) {
	// Ignore results from Gerrit for now.
	if dmResults.isGerritIssue() {
		glog.Infof("Ignoring Gerrit issue  %d/%d for now.", dmResults.Issue, dmResults.Patchset)
		return nil, ingestion.IgnoreResultsFileErr
	}

	var ts time.Time
	var ok bool
	var cacheId = fmt.Sprintf("%d:%d", dmResults.Issue, dmResults.Patchset)
	if ts, ok = g.cache[cacheId]; !ok {
		patchinfo, err := g.getPatchset(dmResults.Issue, dmResults.Patchset)
		if err != nil {
			return nil, err
		}
		ts = patchinfo.Created

		// p.cache is a very crude LRU cache.
		if len(g.cache) > TIMESTAMP_LRU_CACHE_SIZE {
			g.cache = map[string]time.Time{}
		}
		g.cache[cacheId] = ts
	}

	source := fmt.Sprintf("%s/%d", g.review.Url(), dmResults.Issue)
	return &tracedb.CommitID{
		Timestamp: ts.Unix(),
		ID:        strconv.FormatInt(dmResults.Patchset, 10),
		Source:    source,
	}, nil
}

// getPatchset retrieves the patchset. If it does not exist (but the Rietveld issue exists)
// it will return a ingestion.IgnoreResultsFileErr indicating that this input file should be ignored.
func (g *goldTrybotProcessor) getPatchset(issueID int64, patchsetID int64) (*rietveld.Patchset, error) {
	patchinfo, err := g.review.GetPatchset(issueID, patchsetID)
	if err == nil {
		return patchinfo, nil
	}

	// If we can find the issue, check if the patchset has been removed.
	var issueInfo *rietveld.Issue
	if issueInfo, err = g.review.GetIssueProperties(issueID, false); err == nil {
		found := false
		for _, pset := range issueInfo.Patchsets {
			if pset == patchsetID {
				found = true
				break
			}
		}

		// This patchset is no longer available ignore the result file.
		if !found {
			glog.Warningf("Rietveld issue/patchset (%d/%d) does not exist.", issueID, patchsetID)
			return nil, ingestion.IgnoreResultsFileErr
		}
		// We should not reach this point. Investigate manually if we do.
		return nil, fmt.Errorf("Found patchset %d for issue %d, but unable to retrieve it.", patchsetID, issueID)
	}

	return nil, fmt.Errorf("Failed to retrieve trybot issue and patch info for (%d, %d). Got Error: %s", issueID, patchsetID, err)
}

// ExtractIssueInfo returns the issue id and the patchset id for a given commitID.
func ExtractIssueInfo(commitID *tracedb.CommitID, reviewURL string) (string, string) {
	return commitID.Source[strings.LastIndex(commitID.Source, "/")+1:], commitID.ID
}

// GetPrefix returns the filter prefix to search for the given issue and reviewURL.
func GetPrefix(issueID string, reviewURL string) string {
	return fmt.Sprintf("%s/%s", reviewURL, issueID)
}
