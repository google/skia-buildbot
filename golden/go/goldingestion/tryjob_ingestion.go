package goldingestion

import (
	"context"
	"fmt"

	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/golden/go/bbstate"
	"go.skia.org/infra/golden/go/tryjobstore"
	"go.skia.org/infra/golden/go/types"
)

type TryjobProcessorConfig struct {
	GerritURL          string
	CloudProjectID     string
	CDSNamespace       string
	ServiceAccountFile string
	BuildBucketURL     string
	BucketName         string
}

type goldTryjobProcessor struct {
	issueBuildFetcher bbstate.IssueBuildFetcher
	tryjobStore       tryjobstore.TryjobStore
}

func NewGoldTryjobProcessor(config *TryjobProcessorConfig) (ingestion.Processor, error) {
	if config.GerritURL == "" {
		return nil, fmt.Errorf("Missing URL for the Gerrit code review systems. Got value: '%s'", config.GerritURL)
	}

	client, err := auth.NewJWTServiceAccountClient("", config.ServiceAccountFile, nil, gstorage.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("Failed to authenticate service account: %s", err)
	}

	tryjobStore, err := tryjobstore.NewCloudTryjobStore(config.CloudProjectID, config.CDSNamespace, config.ServiceAccountFile)
	if err != nil {
		return nil, fmt.Errorf("Error creating tryjob store: %s", err)
	}

	gerritReview, err := gerrit.NewGerrit(config.GerritURL, "", client)
	if err != nil {
		return nil, err
	}

	bbGerritClient, err := bbstate.NewBuildBucketState(bbstate.DefaultSkiaBuildBucketURL, config.BucketName, client, tryjobStore, gerritReview)
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

	// If we haven't loaded the tryjob then see if we can fetch it from
	// Gerrit and Buildbucket. This should be the exception since tryjobs should
	// be picket up by BuildBucketState as they are added.
	if tryjob == nil {
		if _, tryjob, err = g.issueBuildFetcher.FetchIssueAndTryjob(issueID, dmResults.BuildBucketID); err != nil {
			return err
		}
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

	// Update the database with the results.
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
func (g *goldTryjobProcessor) BatchFinished() error { return nil }
