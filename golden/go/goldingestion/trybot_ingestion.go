package goldingestion

import (
	"context"
	"fmt"
	"net/http"
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
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/bbstate"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/tryjobstore"
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
	bbGerritClient *bbstate.BuildBucketState
	tryjobStore    tryjobstore.TryjobStore

	//    map[issue:patchset] -> timestamp. TODO: REMOVE
	cache      map[string]time.Time
	cacheMutex sync.Mutex
}

func newGoldTrybotProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client) (ingestion.Processor, error) {
	gerritURL := config.ExtraParams[CONFIG_GERRIT_CODE_REVIEW_URL]
	if gerritURL == "" {
		return nil, fmt.Errorf("Missing URL for the Gerrit code review systems. Got value: '%s'", gerritURL)
	}

	svcAccountFile := config.ExtraParams[CONFIG_SERVICE_ACCOUNT_FILE]
	tryjobNamespace := config.ExtraParams[CONFIG_TRYJOB_NAMESPACE]

	client, err := auth.NewJWTServiceAccountClient("", svcAccountFile, nil, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account: %s", err)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(common.PROJECT_ID, tryjobNamespace, svcAccountFile)
	if err != nil {
		return nil, fmt.Errorf("Error creating tryjob store: %s", err)
	}

	gerritReview, err := gerrit.NewGerrit(gerritURL, "", client)
	if err != nil {
		return nil, err
	}

	bbGerritClient, err := bbstate.NewBuildBucketState(bbstate.DefaultSkiaBuildBucketURL, client, tryjobStore, gerritReview)
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
	resultsMap := make(map[string]*tryjobstore.TryjobResult, len(entries))
	for _, entry := range entries {
		key := entry.Params[types.PRIMARY_KEY_FIELD] + string(entry.Value)
		if found, ok := resultsMap[key]; ok {
			found.Params.AddParams(entry.Params)
		} else {
			resultsMap[key] = &tryjobstore.TryjobResult{
				Digest: string(entry.Value),
				Params: paramtools.NewParamSet(entry.Params),
			}
		}
	}

	tjResults := make([]*tryjobstore.TryjobResult, 0, len(resultsMap))
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

	tryjob.Status = tryjobstore.TRYJOB_INGESTED
	if err := g.tryjobStore.UpdateTryjob(issueID, tryjob); err != nil {
		return err
	}

	return nil
}

// See ingestion.Processor interface.
func (g *goldTrybotProcessor) BatchFinished() error { return nil }
