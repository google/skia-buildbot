package ingestion_processors

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/continuous_integration/buildbucket_cis"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
)

const (
	firestoreProjectIDParam = "FirestoreProjectID"
	firestoreNamespaceParam = "FirestoreNamespace"

	codeReviewSystemParam = "CodeReviewSystem"
	gerritURLParam        = "GerritURL"

	continuousIntegrationSystemParam = "ContinuousIntegrationSystem"
	buildBucketNameParam             = "BuildBucketName"
)

type goldTryjobProcessor struct {
	reviewClient      code_review.Client
	integrationClient continuous_integration.Client

	changelistStore clstore.Store
	tryjobStore     tjstore.Store
}

func newGoldTryjobProcessor(_ vcsinfo.VCS, config *sharedconfig.IngesterConfig, client *http.Client, eventBus eventbus.EventBus) (ingestion.Processor, error) {
	crsName := config.ExtraParams[codeReviewSystemParam]
	if strings.TrimSpace(crsName) == "" {
		return nil, skerr.Fmt("missing code review system (e.g. 'gerrit')")
	}

	crs, err := codeReviewSystemFactory(crsName, config, client)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not create client for CRS %q", crsName)
	}

	cisName := config.ExtraParams[continuousIntegrationSystemParam]
	if strings.TrimSpace(cisName) == "" {
		return nil, skerr.Fmt("missing continuous integration system (e.g. 'buildbucket')")
	}

	cis, err := continuousIntegrationSystemFactory(cisName, config, client)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not create client for CIS %q", cisName)
	}

	fsProjectID := config.ExtraParams[firestoreProjectIDParam]
	if strings.TrimSpace(fsProjectID) == "" {
		return nil, skerr.Fmt("missing firestore project id")
	}

	fsNamespace := config.ExtraParams[firestoreNamespaceParam]
	if strings.TrimSpace(fsNamespace) == "" {
		return nil, skerr.Fmt("missing firestore namespace")
	}

	fsClient, err := firestore.NewClient(context.TODO(), fsProjectID, "gold", fsNamespace, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore in project %s, namespace %s", fsProjectID, fsNamespace)
	}

	return &goldTryjobProcessor{
		reviewClient:      crs,
		integrationClient: cis,
		changelistStore:   fs_clstore.New(fsClient, crsName),
		tryjobStore:       fs_tjstore.New(fsClient, cisName),
	}, nil
}

func codeReviewSystemFactory(crsName string, config *sharedconfig.IngesterConfig, client *http.Client) (code_review.Client, error) {
	if crsName == "gerrit" {
		gerritURL := config.ExtraParams[gerritURLParam]
		if strings.TrimSpace(gerritURL) == "" {
			return nil, skerr.Fmt("missing URL for the Gerrit code review system")
		}
		gerritClient, err := gerrit.NewGerrit(gerritURL, "", client)
		if err != nil {
			return nil, skerr.Wrapf(err, "creating gerrit client for %s", gerritURL)
		}
		return gerrit_crs.New(gerritClient), nil
	}
	return nil, skerr.Fmt("CodeReviewSystem %q not recognized", crsName)
}

func continuousIntegrationSystemFactory(cisName string, config *sharedconfig.IngesterConfig, client *http.Client) (continuous_integration.Client, error) {
	if cisName == "buildbucket" {
		bbBucket := config.ExtraParams[buildBucketNameParam]
		if strings.TrimSpace(bbBucket) == "" {
			return nil, skerr.Fmt("missing bucket name for BuildBucket")
		}
		bbClient := buildbucket.NewClient(client)
		return buildbucket_cis.New(bbClient, bbBucket), nil
	}
	return nil, skerr.Fmt("CodeReviewSystem %q not recognized", cisName)
}

func (g *goldTryjobProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	return errors.New("not impl")
}
