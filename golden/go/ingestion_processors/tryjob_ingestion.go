package ingestion_processors

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/dualclstore"
	"go.skia.org/infra/golden/go/clstore/fs_clstore"
	"go.skia.org/infra/golden/go/clstore/sqlclstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/continuous_integration/buildbucket_cis"
	"go.skia.org/infra/golden/go/continuous_integration/simple_cis"
	"go.skia.org/infra/golden/go/expectations/fs_expectationstore"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/dualtjstore"
	"go.skia.org/infra/golden/go/tjstore/fs_tjstore"
	"go.skia.org/infra/golden/go/tjstore/sqltjstore"
)

const (
	firestoreTryJobIngester = "gold_tryjob_fs"
	firestoreProjectIDParam = "FirestoreProjectID"
	firestoreNamespaceParam = "FirestoreNamespace"

	codeReviewSystemsParam     = "CodeReviewSystems"
	gerritURLParam             = "GerritURL"
	gerritInternalURLParam     = "GerritInternalURL"
	githubRepoParam            = "GitHubRepo"
	githubCredentialsPathParam = "GitHubCredentialsPath"

	continuousIntegrationSystemsParam = "ContinuousIntegrationSystems"

	gerritCRS         = "gerrit"
	gerritInternalCRS = "gerrit-internal"
	githubCRS         = "github"
	buildbucketCIS    = "buildbucket"
	cirrusCIS         = "cirrus"
)

// ChangelistFirestore exposes the registration information for an ingester that writes data
// to a Firestore implementation.
func ChangelistFirestore() (id string, constructor ingestion.Constructor) {
	return firestoreTryJobIngester, newModularTryjobProcessor
}

// goldTryjobProcessor implements the ingestion.Processor interface to ingest tryjob results.
type goldTryjobProcessor struct {
	cisClients    map[string]continuous_integration.Client
	reviewSystems []clstore.ReviewSystem
	tryJobStore   tjstore.Store
}

// newModularTryjobProcessor returns an ingestion.Processor which is modular and can support
// different CodeReviewSystems (e.g. "Gerrit", "GitHub") and different ContinuousIntegrationSystems
// (e.g. "BuildBucket", "CirrusCI"). This particular implementation stores the data in Firestore.
func newModularTryjobProcessor(ctx context.Context, _ vcsinfo.VCS, config ingestion.Config, client *http.Client, db *pgxpool.Pool) (ingestion.Processor, error) {
	cisNames := strings.Split(config.ExtraParams[continuousIntegrationSystemsParam], ",")
	if len(cisNames) == 0 {
		return nil, skerr.Fmt("missing CI system (e.g. 'buildbucket')")
	}
	cisClients := make(map[string]continuous_integration.Client, len(cisNames))
	for _, cisName := range cisNames {
		cis, err := continuousIntegrationSystemFactory(cisName, config, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create client for CIS %q", cisName)
		}
		cisClients[cisName] = cis
	}

	fsProjectID := config.ExtraParams[firestoreProjectIDParam]
	if strings.TrimSpace(fsProjectID) == "" {
		return nil, skerr.Fmt("missing firestore project id")
	}

	fsNamespace := config.ExtraParams[firestoreNamespaceParam]
	if strings.TrimSpace(fsNamespace) == "" {
		return nil, skerr.Fmt("missing firestore namespace")
	}

	fsClient, err := firestore.NewClient(ctx, fsProjectID, "gold", fsNamespace, nil)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not init firestore in project %s, namespace %s", fsProjectID, fsNamespace)
	}

	expStore := fs_expectationstore.New(fsClient, nil, fs_expectationstore.ReadOnly)
	if err := expStore.Initialize(ctx); err != nil {
		return nil, skerr.Wrapf(err, "initializing expectation store")
	}

	crsNames := strings.Split(config.ExtraParams[codeReviewSystemsParam], ",")
	if len(crsNames) == 0 {
		return nil, skerr.Fmt("missing CRS (e.g. 'gerrit')")
	}

	var reviewSystems []clstore.ReviewSystem
	for _, crsName := range crsNames {
		crsClient, err := codeReviewSystemFactory(ctx, crsName, config, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create client for CRS %q", crsName)
		}
		fireCS := fs_clstore.New(fsClient, crsName)
		sqlCS := sqlclstore.New(db, crsName)
		reviewSystems = append(reviewSystems, clstore.ReviewSystem{
			ID:     crsName,
			Client: crsClient,
			Store:  dualclstore.New(sqlCS, fireCS),
		})
	}

	fireTS := fs_tjstore.New(fsClient)
	sqlTS := sqltjstore.New(db)
	return &goldTryjobProcessor{
		cisClients:    cisClients,
		tryJobStore:   dualtjstore.New(sqlTS, fireTS),
		reviewSystems: reviewSystems,
	}, nil
}

func codeReviewSystemFactory(ctx context.Context, crsName string, config ingestion.Config, client *http.Client) (code_review.Client, error) {
	if crsName == gerritCRS {
		gerritURL := config.ExtraParams[gerritURLParam]
		if strings.TrimSpace(gerritURL) == "" {
			return nil, skerr.Fmt("missing URL for the Gerrit code review system")
		}
		gerritClient, err := gerrit.NewGerrit(gerritURL, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "creating gerrit client for %s", gerritURL)
		}
		g := gerrit_crs.New(gerritClient)
		email, err := g.LoggedInAs(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "Getting logged in client to gerrit")
		}
		sklog.Infof("Logged into gerrit as %s", email)
		return g, nil
	}
	if crsName == gerritInternalCRS {
		gerritURL := config.ExtraParams[gerritInternalURLParam]
		if strings.TrimSpace(gerritURL) == "" {
			return nil, skerr.Fmt("missing URL for the Gerrit internal code review system")
		}
		gerritClient, err := gerrit.NewGerrit(gerritURL, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "creating gerrit client for %s", gerritURL)
		}
		g := gerrit_crs.New(gerritClient)
		email, err := g.LoggedInAs(ctx)
		if err != nil {
			return nil, skerr.Wrapf(err, "Getting logged in client to gerrit-internal")
		}
		sklog.Infof("Logged into gerrit-internal as %s", email)
		return g, nil
	}
	if crsName == githubCRS {
		githubRepo := config.ExtraParams[githubRepoParam]
		if strings.TrimSpace(githubRepo) == "" {
			return nil, skerr.Fmt("missing repo for the GitHub code review system")
		}
		githubCredPath := config.ExtraParams[githubCredentialsPathParam]
		if strings.TrimSpace(githubCredPath) == "" {
			return nil, skerr.Fmt("missing credentials path for the GitHub code review system")
		}
		gBody, err := ioutil.ReadFile(githubCredPath)
		if err != nil {
			return nil, skerr.Wrapf(err, "reading githubToken in %s", githubCredPath)
		}
		gToken := strings.TrimSpace(string(gBody))
		githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
		c := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
		return github_crs.New(c, githubRepo), nil
	}
	return nil, skerr.Fmt("CodeReviewSystem %q not recognized", crsName)
}

func continuousIntegrationSystemFactory(cisName string, _ ingestion.Config, client *http.Client) (continuous_integration.Client, error) {
	if cisName == buildbucketCIS {
		bbClient := buildbucket.NewClient(client)
		return buildbucket_cis.New(bbClient), nil
	}
	if cisName == cirrusCIS {
		return simple_cis.New(cisName), nil
	}
	return nil, skerr.Fmt("ContinuousIntegrationSystem %q not recognized", cisName)
}

// Process implements the Processor interface.
func (g *goldTryjobProcessor) Process(ctx context.Context, rf ingestion.ResultFileLocation) error {
	defer metrics2.FuncTimer().Stop()
	gr, err := processGoldResults(ctx, rf)
	if err != nil {
		sklog.Errorf("Error processing result: %s", err)
		return ingestion.IgnoreResultsFileErr
	}

	clID := ""
	psOrder := 0
	psID := ""
	crs := gr.CodeReviewSystem
	if crs == "" {
		// Default to Gerrit; TODO(kjlubick) who uses this?
		sklog.Warningf("Using default CRS (this may go away soon)")
		crs = gerritCRS
	}

	system, ok := g.getCodeReviewSystem(crs)
	if !ok {
		sklog.Warningf("Result %s said it was for crs %q, which we aren't configured for", rf.Name(), crs)
		return ingestion.IgnoreResultsFileErr
	}
	clID = gr.ChangelistID
	psOrder = gr.PatchsetOrder
	psID = gr.PatchsetID

	tjID := ""
	cisName := gr.ContinuousIntegrationSystem
	if cisName == "" {
		// Default to BuildBucket; TODO(kjlubick) who uses this?
		sklog.Warningf("Using default CIS (this may go away soon)")
		cisName = buildbucketCIS
	}
	var cisClient continuous_integration.Client
	if ci, ok := g.cisClients[cisName]; ok {
		tjID = gr.TryJobID
		cisClient = ci
	} else {
		sklog.Warningf("Result %s said it was for cis %q, but this ingester wasn't configured for it", rf.Name(), cisName)
		// We only support one CRS and one CIS at the moment, but if needed, we can have
		// multiple configured and pivot to the one we need.
		return ingestion.IgnoreResultsFileErr
	}

	// Fetch CL from clstore if we have seen it before, from CRS if we have not.
	cl, err := system.Store.GetChangelist(ctx, clID)
	if err == clstore.ErrNotFound {
		cl, err = system.Client.GetChangelist(ctx, clID)
		if err == code_review.ErrNotFound {
			sklog.Warningf("Unknown %s CL with id %q", crs, clID)
			// Try again later - maybe the input was created before the CL?
			return ingestion.IgnoreResultsFileErr
		} else if err != nil {
			return skerr.Wrapf(err, "fetching CL from %s with id %q", crs, clID)
		}
		// This is a new CL, but we'll be storing it to the clstore down below when
		// we confirm that the TryJob is valid.
		cl.Updated = time.Time{} // store a sentinel value to CL.
	} else if err != nil {
		return skerr.Wrapf(err, "fetching CL from clstore with id %q", clID)
	}

	ps, err := g.getPatchset(ctx, system, psOrder, psID, clID)
	if err != nil {
		// Do not wrap this error - this returns IgnoreResultsFileErr sometimes.
		return err
	}

	combinedID := tjstore.CombinedPSID{CL: clID, PS: ps.SystemID, CRS: crs}

	// We now need to 1) verify the TryJob is valid (either we've seen it before and know it's valid
	// or we check now with the CIS) and 2) update the Changelist's timestamp and store it to
	// clstore. This "refreshes" the Changelist, making it appear higher up on search results, etc.
	_, err = g.tryJobStore.GetTryJob(ctx, tjID, cisName)
	var tj continuous_integration.TryJob
	writeTryJob := false
	if err == tjstore.ErrNotFound {
		tj, err = cisClient.GetTryJob(ctx, tjID)
		if err == tjstore.ErrNotFound {
			sklog.Warningf("Unknown %s Tryjob with id %q", cisName, tjID)
			// Try again later - maybe there's some lag with the Integration System?
			return ingestion.IgnoreResultsFileErr
		} else if err != nil {
			return skerr.Wrapf(err, "fetching tryjob from %s with id %q", cisName, tjID)
		}
		writeTryJob = true
		// If we are seeing that a CL was marked as Abandoned, it probably means the CL was
		// re-opened. If this is incorrect (e.g. TryJob was triggered, CL was abandoned, commenter
		// noticed CL was abandoned, and then the TryJob results started being processed), this
		// is fine to mark it as Open, because commenter will correctly mark it as abandoned again.
		// This approach makes fewer queries to the CodeReviewSystem than, for example, querying
		// the CRS *here* if the CL is really open. Keeping CRS queries to a minimum is important,
		// because our quota of them is not high enough to potentially check a CL is abandoned for
		// every TryJobResult that is being streamed in.
		if cl.Status == code_review.Abandoned {
			cl.Status = code_review.Open
		}
	} else if err != nil {
		return skerr.Wrapf(err, "fetching TryJob with id %s", tjID)
	}
	// In the SQL implementation, we need to create the CL first because of foreign key constraints.
	if cl.Updated.IsZero() {
		sklog.Debugf("First time seeing CL %s_%s", system.ID, cl.SystemID)
		if err = system.Store.PutChangelist(ctx, cl); err != nil {
			return skerr.Wrapf(err, "initially storing %s CL with id %q to clstore", system.ID, clID)
		}
	}

	defer shared.NewMetricsTimer("put_tryjobstore_entries").Stop()
	// Store the results from the file.
	if err := system.Store.PutPatchset(ctx, ps); err != nil {
		return skerr.Wrapf(err, "could not store PS %s of %s CL %q to clstore", psID, system.ID, clID)
	}
	if writeTryJob {
		if err := g.tryJobStore.PutTryJob(ctx, combinedID, tj); err != nil {
			return skerr.Wrapf(err, "storing tryjob %q to tryjobstore", tjID)
		}
	}
	tjr := toTryJobResults(gr, tjID, cisName)
	err = g.tryJobStore.PutResults(ctx, combinedID, rf.Name(), tjr, time.Now())
	if err != nil {
		return skerr.Wrapf(err, "putting %d results for CL %s, PS %d (%s), TJ %s, file %s", len(tjr), clID, psOrder, psID, tjID, rf.Name())
	}

	// Be sure to update this time now, so that other processes can use the cl.Update timestamp
	// to determine if any changes have happened to the CL or any children PSes in a given time
	// period.
	cl.Updated = time.Now()
	if err = system.Store.PutChangelist(ctx, cl); err != nil {
		return skerr.Wrapf(err, "updating %s CL with id %q to clstore", system.ID, clID)
	}
	return nil
}

// getPatchset looks up a Patchset either by id or order from our changelistStore. If it's not
// there, it looks it up from the CRS and then stores it to the changelistStore before returning it.
func (g *goldTryjobProcessor) getPatchset(ctx context.Context, system clstore.ReviewSystem, psOrder int, psID, clID string) (code_review.Patchset, error) {
	// Try looking up patchset by ID first, then fall back to order.
	if psID != "" {
		// Fetch PS from clstore if we have seen it before, from CRS if we have not.
		ps, err := system.Store.GetPatchset(ctx, clID, psID)
		if err == clstore.ErrNotFound {
			ps, err := system.Client.GetPatchset(ctx, clID, psID, 0)
			if err != nil {
				sklog.Warningf("Unknown %s PS %s for CL %q: %s", system.ID, psID, clID, err)
				// Try again later - maybe the input was created before the CL uploaded its PS?
				return code_review.Patchset{}, ingestion.IgnoreResultsFileErr
			}
			return ps, nil
		} else if err != nil {
			return code_review.Patchset{}, skerr.Wrapf(err, "fetching PS from clstore with id %s for CL %q", psID, clID)
		}
		// already found the PS in the store
		return ps, nil
	}
	// Fetch PS from clstore if we have seen it before, from CRS if we have not.
	ps, err := system.Store.GetPatchsetByOrder(ctx, clID, psOrder)
	if err == clstore.ErrNotFound {
		ps, err := system.Client.GetPatchset(ctx, clID, "", psOrder)
		if err != nil {
			sklog.Warningf("Unknown %s PS with order %d for CL %q", system.ID, psOrder, clID)
			// Try again later - maybe the input was created before the CL uploaded its PS?
			return code_review.Patchset{}, ingestion.IgnoreResultsFileErr
		}
		return ps, nil
	} else if err != nil {
		return code_review.Patchset{}, skerr.Wrapf(err, "fetching PS from clstore with order %d for CL %q", psOrder, clID)
	}
	// already found the PS in the store
	return ps, nil
}

// toTryJobResults converts the JSON file to a slice of TryJobResult.
func toTryJobResults(j *jsonio.GoldResults, tjID, cisName string) []tjstore.TryJobResult {
	var tjr []tjstore.TryJobResult
	for _, r := range j.Results {
		tjr = append(tjr, tjstore.TryJobResult{
			System:       cisName,
			TryjobID:     tjID,
			GroupParams:  j.Key,
			ResultParams: r.Key,
			Options:      r.Options,
			Digest:       r.Digest,
		})
	}
	return tjr
}

// getCodeReviewSystem returns the ReviewSystem associated with the crs, or false if there was no
// match.
func (g *goldTryjobProcessor) getCodeReviewSystem(crs string) (clstore.ReviewSystem, bool) {
	var system clstore.ReviewSystem
	found := false
	for _, rs := range g.reviewSystems {
		if rs.ID == crs {
			system = rs
			found = true
		}
	}
	return system, found
}
