package goldingestion

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/golden/go/trybotstore"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/config"
)

const (
	CONFIG_RIETVELD_CODE_REVIEW_URL = "RietveldCodeReviewURL"
	CONFIG_GERRIT_CODE_REVIEW_URL   = "GerritCodeReviewURL"
	CONFIG_TRYJOB_NAMESPACE         = "TryjobDatastoreNameSpace"
	CONFIG_SERVICE_ACCOUNT_FILE     = "ServiceAccountFile"

	TIMESTAMP_LRU_CACHE_SIZE = 1000
)

func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYBOT, newGoldTrybotProcessor)
}

type goldTrybotProcessor struct {
	*goldProcessor
	gerritReview *gerrit.Gerrit
	tryjobStore  trybotstore.TrybotStore

	//    map[issue:patchset] -> timestamp.
	cache      map[string]time.Time
	cacheMutex sync.Mutex
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	processor, err := newGoldProcessor(vcs, config, client)
	if err != nil {
		return nil, err
	}

	svcAccount := config.ExtraParams[CONFIG_SERVICE_ACCOUNT_FILE]
	tryjobNamespace := config.ExtraParams[CONFIG_TRYJOB_NAMESPACE]

	tryjobStore, err := trybotstore.NewCloudTrybotStore(common.PROJECT_ID, tryjobNamespace, svcAccount)
	if err != nil {
		return nil, fmt.Errorf("Error creating tryjob store: %s", err)
	}

	ingestionStore, err := NewIngestionStore(config.ExtraParams[CONFIG_TRACESERVICE])
	if err != nil {
		return nil, fmt.Errorf("Unable to open ingestion store: %s", err)
	}

	// Get the underlying goldProcessor.
	gProcessor := processor.(*goldProcessor)
	gProcessor.ingestionStore = ingestionStore

	gerritURL := config.ExtraParams[CONFIG_GERRIT_CODE_REVIEW_URL]
	if gerritURL == "" {
		return nil, fmt.Errorf("Missing URL for the Gerrit code review systems. Got value: '%s'", gerritURL)
	}

	gerritReview, err := gerrit.NewGerrit(gerritURL, "", nil)
	if err != nil {
		return nil, err
	}

	ret := &goldTrybotProcessor{
		goldProcessor: gProcessor,
		gerritReview:  gerritReview,
		tryjobStore:   tryjobStore,
		cache:         map[string]time.Time{},
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
	source := g.gerritReview.Url(dmResults.Issue)
	return &tracedb.CommitID{
		Timestamp: ts.Unix(),
		ID:        strconv.FormatInt(dmResults.Patchset, 10),
		Source:    source,
	}, nil
}

// getCreatedTimeStamp returns the timestamp of the patchset contained in
// DMResult either from Gerrit or Rietveld.
func (g *goldTrybotProcessor) getCreatedTimeStamp(dmResults *DMResults) (time.Time, error) {
	issueProps, err := g.gerritReview.GetIssueProperties(dmResults.Issue)
	if err != nil {
		return time.Time{}, err
	}

	if len(issueProps.Patchsets) < int(dmResults.Patchset) {
		return time.Time{}, fmt.Errorf("Given patchset (%d) number is not available from Gerrit. Only found %d patchsets.", dmResults.Patchset, len(issueProps.Patchsets))
	}

	return issueProps.Patchsets[dmResults.Patchset-1].Created, nil
}

// // ExtractIssueInfo returns the issue id and the patchset id for a given commitID.
// func ExtractIssueInfo(commitID *tracedb.CommitID, gerritReview *gerrit.Gerrit) (string, string) {
// 	issue, _ := gerritReview.ExtractIssue(commitID.Source)
// 	return issue, commitID.ID
// }
