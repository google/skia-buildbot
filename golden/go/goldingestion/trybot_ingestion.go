package goldingestion

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	tracedb "go.skia.org/infra/go/trace/db"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/bbtrybot"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/trybotstore"
	"go.skia.org/infra/golden/go/types"
)

const (
	CONFIG_GERRIT_CODE_REVIEW_URL = "GerritCodeReviewURL"
	CONFIG_TRYJOB_NAMESPACE       = "TryjobDatastoreNameSpace"
	CONFIG_SERVICE_ACCOUNT_FILE   = "ServiceAccountFile"
	TIMESTAMP_LRU_CACHE_SIZE      = 1000
)

func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYBOT, newGoldTrybotProcessor)
}

type goldTrybotProcessor struct {
	bbGerritClient *bbtrybot.TrybotState
	tryjobStore    trybotstore.TrybotStore

	//    map[issue:patchset] -> timestamp. TODO: REMOVE
	cache      map[string]time.Time
	cacheMutex sync.Mutex
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	gerritURL := config.ExtraParams[CONFIG_GERRIT_CODE_REVIEW_URL]
	if gerritURL == "" {
		return nil, fmt.Errorf("Missing URL for the Gerrit code review systems. Got value: '%s'", gerritURL)
	}

	svcAccount := config.ExtraParams[CONFIG_SERVICE_ACCOUNT_FILE]
	tryjobNamespace := config.ExtraParams[CONFIG_TRYJOB_NAMESPACE]

	client, err := auth.NewJWTServiceAccountClient("", svcAccount, nil, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	tryjobStore, err := trybotstore.NewCloudTrybotStore(common.PROJECT_ID, tryjobNamespace, client)
	if err != nil {
		return nil, fmt.Errorf("Error creating tryjob store: %s", err)
	}

	gerritReview, err := gerrit.NewGerrit(gerritURL, "", client)
	if err != nil {
		return nil, err
	}

	bbGerritClient, err := bbtrybot.NewTrybotState(client, tryjobStore, gerritReview)
	if err != nil {
		return nil, err
	}
	return &goldTrybotProcessor{
		bbGerritClient: bbGerritClient,
		tryjobStore:    tryjobStore,
		cache:          map[string]time.Time{},
	}, nil
}

// See ingestion.Processor interface.
func (g *goldTrybotProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	dmResults, err := processDMResults(resultsFile)
	if err != nil {
		return err
	}

	entries, err := dmResults.getTraceDBEntries()
	if err != nil {
		return err
	}

	// Convert to a trybotstore.TryjobResult slice by aggregating parameters for each test/digest pair.
	resultsMap := make(map[string]*trybotstore.TryjobResult, len(entries))
	for _, entry := range entries {
		key := entry.Params[types.PRIMARY_KEY_FIELD] + string(entry.Value)
		if found, ok := resultsMap[key]; ok {
			found.Params.AddParams(entry.Params)
		} else {
			resultsMap[key] = &trybotstore.TryjobResult{
				Digest: string(entry.Value),
				Params: paramtools.NewParamSet(entry.Params),
			}
		}
	}

	tjResults := make([]*trybotstore.TryjobResult, 0, len(resultsMap))
	for _, result := range resultsMap {
		tjResults = append(tjResults, result)
	}

	// Save the results to the trybot store.
	issueID := dmResults.Issue
	tryjob, err := g.tryjobStore.GetTryjob(issueID, dmResults.BuildBucketID)
	if err != nil {
		return err
	}

	if tryjob == nil {
		if ok, err := g.bbGerritClient.LoadGerritIssue(issueID); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("Unable to find issue %d in Gerrit.", issueID)
		}
	}

	if err := g.tryjobStore.UpdateTryjobResult(tryjob, tjResults); err != nil {
		return err
	}

	tryjob.Status = trybotstore.TRYJOB_INGESTED
	if err := g.tryjobStore.UpdateTryjob(issueID, tryjob); err != nil {
		return err
	}

	return nil
}

// See ingestion.Processor interface.
func (g *goldTrybotProcessor) BatchFinished() error { return nil }

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
