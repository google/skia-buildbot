package goldingestion

import (
	"fmt"

	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/golden/go/bbstate"
	"go.skia.org/infra/golden/go/tryjobstore"
)

type TryjobProcessorConfig struct {
	GerritURL          string
	CloudProjectID     string
	CDSNamespace       string
	ServiceAccountFile string
	BuildBucketURL     string
	BucketName         string
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
