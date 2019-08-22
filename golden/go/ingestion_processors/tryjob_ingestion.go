package ingestion_processors

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sharedconfig"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/continuous_integration/buildbucket_cis"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
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

	gerritCRS      = "gerrit"
	buildbucketCIS = "buildbucket"
)

type goldTryjobProcessor struct {
	reviewClient      code_review.Client
	integrationClient continuous_integration.Client

	changelistStore clstore.Store
	tryjobStore     tjstore.Store

	crsName string
	cisName string
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
		crsName:           crsName,
		cisName:           cisName,
	}, nil
}

func codeReviewSystemFactory(crsName string, config *sharedconfig.IngesterConfig, client *http.Client) (code_review.Client, error) {
	if crsName == gerritCRS {
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
	if cisName == buildbucketCIS {
		bbBucket := config.ExtraParams[buildBucketNameParam]
		if strings.TrimSpace(bbBucket) == "" {
			return nil, skerr.Fmt("missing bucket name for BuildBucket")
		}
		bbClient := buildbucket.NewClient(client)
		return buildbucket_cis.New(bbClient, bbBucket), nil
	}
	return nil, skerr.Fmt("ContinuousIntegrationSystem %q not recognized", cisName)
}

func (g *goldTryjobProcessor) Process(ctx context.Context, rf ingestion.ResultFileLocation) error {
	defer metrics2.FuncTimer().Stop()
	gr, err := processGoldResults(rf)
	if err != nil {
		sklog.Errorf("Error processing result: %s", err)
		return ingestion.IgnoreResultsFileErr
	}

	clID := ""
	psID := ""
	crs := gr.CodeReviewSystem
	if crs == "" || crs == g.crsName {
		// Default to Gerrit
		crs = gerritCRS
		clID = strconv.FormatInt(gr.GerritChangeListID, 10)
		psID = strconv.FormatInt(gr.GerritPatchSet, 10)
	} else {
		sklog.Warningf("Result %s said it was for crs %q, but this ingester is configured for %s", rf.Name(), crs, g.crsName)
		// We only support one CRS and one CIS at the moment, but if needed, we can have
		// multiple configured and pivot to the one we need.
		return ingestion.IgnoreResultsFileErr
	}

	tjID := ""
	cis := gr.ContinuousIntegrationSystem
	if cis == "" || cis == g.cisName {
		// Default to BuildBucket
		cis = buildbucketCIS
		tjID = strconv.FormatInt(gr.BuildBucketID, 10)
	} else {
		sklog.Warningf("Result %s said it was for cis %q, but this ingester is configured for %s", rf.Name(), cis, g.cisName)
		// We only support one CRS and one CIS at the moment, but if needed, we can have
		// multiple configured and pivot to the one we need.
		return ingestion.IgnoreResultsFileErr
	}

	// Fetch CL from clstore if we have seen it before, from CRS if we have not.
	_, err = g.changelistStore.GetChangeList(ctx, clID)
	if err == clstore.ErrNotFound {
		cl, err := g.reviewClient.GetChangeList(ctx, clID)
		if err == code_review.ErrNotFound {
			sklog.Warningf("Unknown %s CL with id %q", crs, clID)
			// Try again later - maybe the input was created before the CL?
			return ingestion.IgnoreResultsFileErr
		} else if err != nil {
			return skerr.Wrapf(err, "fetching CL from %s with id %q", crs, clID)
		}
		err = g.changelistStore.PutChangeList(ctx, cl)
		if err != nil {
			return skerr.Wrapf(err, "storing CL with id %q to clstore", clID)
		}
	} else if err != nil {
		return skerr.Wrapf(err, "fetching CL from clstore with id %q", clID)
	}

	// Fetch PS from clstore if we have seen it before, from CRS if we have not.
	_, err = g.changelistStore.GetPatchSet(ctx, clID, psID)
	if err == clstore.ErrNotFound {
		xps, err := g.reviewClient.GetPatchSets(ctx, clID)
		if err != nil {
			return skerr.Wrapf(err, "could not get patchsets for %s cl %s", crs, clID)
		}
		// It should be ok to put any PatchSets we've seen before - they should be immutable.
		found := false
		for _, p := range xps {
			err := g.changelistStore.PutPatchSet(ctx, clID, p)
			if err != nil {
				return skerr.Wrapf(err, "could not store PS %q of CL %q to clstore", psID, clID)
			}
			// Only store PatchSets up to the latest one we know has data
			if p.SystemID == psID {
				found = true
				break
			}
		}
		if !found {
			sklog.Warningf("Unknown %s PS with id %q for CL %q", crs, psID, clID)
			// Try again later - maybe the input was created before the CL uploaded its PS?
			return ingestion.IgnoreResultsFileErr
		}
	} else if err != nil {
		return skerr.Wrapf(err, "fetching PS from clstore with id %q for CL %q", psID, clID)
	}

	combinedID := tjstore.CombinedPSID{CL: clID, PS: psID, CRS: crs}

	_, err = g.tryjobStore.GetTryJob(ctx, tjID)
	if err == tjstore.ErrNotFound {
		tj, err := g.integrationClient.GetTryJob(ctx, tjID)
		if err == tjstore.ErrNotFound {
			sklog.Warningf("Unknown %s Tryjob with id %q", cis, tjID)
			// Try again later - maybe there's some lag with the Integration System?
			return ingestion.IgnoreResultsFileErr
		} else if err != nil {
			return skerr.Wrapf(err, "fetching tryjob from %s with id %q", cis, tjID)
		}
		err = g.tryjobStore.PutTryJob(ctx, combinedID, tj)
		if err != nil {
			return skerr.Wrapf(err, "storing tryjob %q to tryjobstore", tjID)
		}
	}

	defer shared.NewMetricsTimer("put_tryjobstore_entries").Stop()

	// Store the results from the file.
	tjr := toTryJobResults(gr)
	err = g.tryjobStore.PutResults(ctx, combinedID, tjr)
	if err != nil {
		return skerr.Wrapf(err, "putting %d results for CL %s, PS %s, TJ %s, file %s", len(tjr), clID, psID, tjID, rf.Name())
	}

	return nil
}

// toTryJobResults converts the JSON file to a slize of TryJobResult.
func toTryJobResults(j *jsonio.GoldResults) []tjstore.TryJobResult {
	var tjr []tjstore.TryJobResult
	for _, r := range j.Results {
		tjr = append(tjr, tjstore.TryJobResult{
			GroupParams:  j.Key,
			ResultParams: r.Key,
			Options:      r.Options,
			Digest:       r.Digest,
		})
	}
	return tjr
}
