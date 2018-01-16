package goldingestion

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/human"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/bbstate"
	"go.skia.org/infra/golden/go/config"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
	gstorage "google.golang.org/api/storage/v1"
)

// Define configuration options to be used in the config file under
// ExtraParams.
const (
	CONFIG_GERRIT_CODE_REVIEW_URL     = "GerritCodeReviewURL"
	CONFIG_TRYJOB_NAMESPACE           = "TryjobDatastoreNameSpace"
	CONFIG_SERVICE_ACCOUNT_FILE       = "ServiceAccountFile"
	CONFIG_BUILD_BUCKET_URL           = "BuildBucketURL"
	CONFIG_BUILD_BUCKET_NAME          = "BuildBucketName"
	CONFIG_BUILD_BUCKET_POLL_INTERVAL = "BuildBucketPollInterval"
	CONFIG_BUILD_BUCKET_TIME_WINDOW   = "BuildBucketTimeWindow"
)

// Register the ingestion Processor with the ingestion framework.
func init() {
	ingestion.Register(config.CONSTRUCTOR_GOLD_TRYJOB, newGoldTryjobProcessor)
}

// goldTryjobProcessor implements the ingestion.Processor interface to ingest
// tryjob results.
type goldTryjobProcessor struct {
	issueBuildFetcher bbstate.IssueBuildFetcher
	tryjobStore       tryjobstore.TryjobStore
}

// newGoldTryjobProcessor implementes the ingestion.Constructor function.
func newGoldTryjobProcessor(vcs vcsinfo.VCS, config *sharedconfig.IngesterConfig, clientx *http.Client) (ingestion.Processor, error) {
	sklog.Infof("Creating tryjob processor.")
	gerritURL, ok := config.ExtraParams[CONFIG_GERRIT_CODE_REVIEW_URL]
	if !ok {
		return nil, fmt.Errorf("Missing URL for the Gerrit code review systems. Got value: '%s'", gerritURL)
	}

	// Get the config options.
	svcAccountFile := config.ExtraParams[CONFIG_SERVICE_ACCOUNT_FILE]
	sklog.Infof("Got service accoutn file '%s'", svcAccountFile)

	pollInterval, err := parseDuration(config.ExtraParams[CONFIG_BUILD_BUCKET_POLL_INTERVAL], bbstate.DefaultPollInterval)
	if err != nil {
		return nil, err
	}

	timeWindow, err := parseDuration(config.ExtraParams[CONFIG_BUILD_BUCKET_TIME_WINDOW], bbstate.DefaultTimeWindow)
	if err != nil {
		return nil, err
	}

	tryjobNamespace, ok := config.ExtraParams[CONFIG_TRYJOB_NAMESPACE]
	if !ok {
		return nil, fmt.Errorf("Missing cloud datastore namespace for tryjob data.")
	}

	buildBucketURL := config.ExtraParams[CONFIG_BUILD_BUCKET_URL]
	buildBucketName := config.ExtraParams[CONFIG_BUILD_BUCKET_NAME]
	if (buildBucketURL == "") || (buildBucketName == "") {
		return nil, fmt.Errorf("BuildBucketName and BuildBucketURL must not be empty.")
	}

	client, err := auth.NewJWTServiceAccountClient("", svcAccountFile, nil, gstorage.CloudPlatformScope, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate service account: %s", err)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(common.PROJECT_ID, tryjobNamespace, svcAccountFile)
	if err != nil {
		return nil, fmt.Errorf("Error creating tryjob store: %s", err)
	}

	gerritReview, err := gerrit.NewGerrit(gerritURL, "", client)
	if err != nil {
		return nil, err
	}

	bbConf := &bbstate.Config{
		BuildBucketURL:  buildBucketURL,
		BuildBucketName: buildBucketName,
		Client:          client,
		TryjobStore:     tryjobStore,
		GerritClient:    gerritReview,
		PollInterval:    pollInterval,
		TimeWindow:      timeWindow,
	}

	bbGerritClient, err := bbstate.NewBuildBucketState(bbConf)
	if err != nil {
		return nil, err
	}
	return &goldTryjobProcessor{
		issueBuildFetcher: bbGerritClient,
		tryjobStore:       tryjobStore,
	}, nil
}

// See ingestion.Processor interface.
func (g *goldTryjobProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	dmResults, err := processDMResults(resultsFile)
	if err != nil {
		return err
	}

	// Make sure we have an issue, patchset and a buildbucket id.
	if (dmResults.Issue <= 0) || (dmResults.Patchset <= 0) || (dmResults.BuildBucketID <= 0) {
		return fmt.Errorf("Invalid data. issue, patchset and buildbucket id must be > 0. Got (%d, %d, %d).", dmResults.Issue, dmResults.Patchset, dmResults.BuildBucketID)
	}

	entries, err := dmResults.getTraceDBEntries()
	if err != nil {
		return err
	}

	// Save the results to the trybot store.
	issueID := dmResults.Issue
	tryjob, err := g.tryjobStore.GetTryjob(issueID, dmResults.BuildBucketID)
	if err != nil {
		return err
	}

	// Fetch the issue and check if the trybot is contained.
	issue, err := g.tryjobStore.GetIssue(issueID, false, nil)
	if err != nil {
		return sklog.FmtErrorf("Unable to retrieve issue %d to process file %s. Got error: %s", issueID, resultsFile.Name(), err)
	}

	// If we haven't loaded the tryjob then see if we can fetch it from
	// Gerrit and Buildbucket. This should be the exception since tryjobs should
	// be picket up by BuildBucketState as they are added.
	if (tryjob == nil) || (issue == nil) || !issue.HasPatchset(tryjob.PatchsetID) {
		if issue, tryjob, err = g.issueBuildFetcher.FetchIssueAndTryjob(issueID, dmResults.BuildBucketID); err != nil {
			return err
		}
	}

	// Convert to a trybotstore.TryjobResult slice by aggregating parameters for each test/digest pair.
	resultsMap := make(map[string]*tryjobstore.TryjobResult, len(entries))
	for _, entry := range entries {
		testName := entry.Params[types.PRIMARY_KEY_FIELD]
		key := testName + string(entry.Value)
		if found, ok := resultsMap[key]; ok {
			found.Params.AddParams(entry.Params)
		} else {
			resultsMap[key] = &tryjobstore.TryjobResult{
				TestName: testName,
				Digest:   string(entry.Value),
				Params:   paramtools.NewParamSet(entry.Params),
			}
		}
	}

	tjResults := make([]*tryjobstore.TryjobResult, 0, len(resultsMap))
	for _, result := range resultsMap {
		tjResults = append(tjResults, result)
	}

	// Update the database with the results.
	if err := g.tryjobStore.UpdateTryjobResult(tryjob, tjResults); err != nil {
		return err
	}

	tryjob.Status = tryjobstore.TRYJOB_INGESTED
	return g.tryjobStore.UpdateTryjob(issueID, tryjob)
}

// See ingestion.Processor interface.
func (g *goldTryjobProcessor) BatchFinished() error { return nil }

// parseDuration parses the given duration. If strVal is empty the default value
// will be returned. If the given duration is invalid an error will be returned.
func parseDuration(strVal string, defaultVal time.Duration) (time.Duration, error) {
	if strVal == "" {
		return defaultVal, nil
	}
	return human.ParseDuration(strVal)
}
