package ingestion_processors

import (
	"context"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/clstore/sqlclstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/continuous_integration/buildbucket_cis"
	"go.skia.org/infra/golden/go/continuous_integration/simple_cis"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/shared"
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/sqltjstore"
)

const (
	TryjobSQLConfig = "gold_tryjob_fs" // TODO(kjlubick) update constant

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

// goldTryjobProcessor implements the ingestion.Processor interface to ingest tryjob results.
type goldTryjobProcessor struct {
	cisClients    map[string]continuous_integration.Client
	reviewSystems []clstore.ReviewSystem
	tryJobStore   tjstore.Store
	source        ingestion.Source
}

// TryjobSQL returns an ingestion.Processor which is modular and can support
// different CodeReviewSystems (e.g. "Gerrit", "GitHub") and different ContinuousIntegrationSystems
// (e.g. "BuildBucket", "CirrusCI"). This particular implementation stores the data in SQL.
func TryjobSQL(ctx context.Context, config ingestion.Config, client *http.Client, db *pgxpool.Pool, src ingestion.Source) (ingestion.Processor, error) {
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
		sqlCS := sqlclstore.New(db, crsName)
		reviewSystems = append(reviewSystems, clstore.ReviewSystem{
			ID:     crsName,
			Client: crsClient,
			Store:  sqlCS,
		})
	}

	sqlTS := sqltjstore.New(db)
	return &goldTryjobProcessor{
		cisClients:    cisClients,
		tryJobStore:   sqlTS,
		reviewSystems: reviewSystems,
		source:        src,
	}, nil
}

// HandlesFile returns true if the configured source handles this file.
func (g *goldTryjobProcessor) HandlesFile(name string) bool {
	return g.source.HandlesFile(name)
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
func (g *goldTryjobProcessor) Process(ctx context.Context, fileName string) error {
	ctx, span := trace.StartSpan(ctx, "ingestion_SQLTryJobProcess")
	defer span.End()
	r, err := g.source.GetReader(ctx, fileName)
	if err != nil {
		return skerr.Wrap(err)
	}
	gr, err := processGoldResults(ctx, r)
	if err != nil {
		return skerr.Wrapf(err, "could not process file %s from source %s", fileName, g.source)
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
		return skerr.Fmt("File %s said it was for crs %q, which we aren't configured for", fileName, crs)
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
		return skerr.Fmt("File %s said it was for cis %q, but this ingester wasn't configured for it", fileName, cisName)
	}

	// Fetch CL from clstore if we have seen it before, from CRS if we have not.
	cl, err := system.Store.GetChangelist(ctx, clID)
	if err == clstore.ErrNotFound {
		cl, err = system.Client.GetChangelist(ctx, clID)
		if err == code_review.ErrNotFound {
			sklog.Warningf("Unknown %s CL with id %q", crs, clID)
			// Try again later
			return ingestion.ErrRetryable
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
		return skerr.Wrap(err)
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
			return ingestion.ErrRetryable
		} else if err != nil {
			sklog.Errorf("fetching tryjob from %s with id %q: %s", cisName, tjID, err)
			return ingestion.ErrRetryable
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
		sklog.Errorf("fetching TryJob from store with id %q: %s", tjID, err)
		return ingestion.ErrRetryable
	}
	// In the SQL implementation, we need to create the CL first because of foreign key constraints.
	if cl.Updated.IsZero() {
		sklog.Debugf("First time seeing CL %s_%s", system.ID, cl.SystemID)
		if err = system.Store.PutChangelist(ctx, cl); err != nil {
			sklog.Errorf("Initially storing %s CL with id %q to clstore: %s", system.ID, clID, err)
			return ingestion.ErrRetryable
		}
	}

	defer shared.NewMetricsTimer("put_tryjobstore_entries").Stop()
	// Store the results from the file.
	if err := system.Store.PutPatchset(ctx, ps); err != nil {
		sklog.Errorf("Could not store PS %s of %s CL %q to clstore: %s", psID, system.ID, clID, err)
		return ingestion.ErrRetryable
	}
	if writeTryJob {
		if err := g.tryJobStore.PutTryJob(ctx, combinedID, tj); err != nil {
			sklog.Errorf("Storing tryjob %q to tryjobstore: %s", tjID, err)
			return ingestion.ErrRetryable
		}
	}
	tjr := toTryJobResults(gr, tjID, cisName)
	err = g.tryJobStore.PutResults(ctx, combinedID, fileName, tjr, time.Now())
	if err != nil {
		sklog.Errorf("Putting %d results for CL %s, PS %d (%s), TJ %s, file %s: %s", len(tjr), clID, psOrder, psID, tjID, fileName, err)
		return ingestion.ErrRetryable
	}

	// Be sure to update this time now, so that other processes can use the cl.Update timestamp
	// to determine if any changes have happened to the CL or any children PSes in a given time
	// period.
	cl.Updated = time.Now()
	if err = system.Store.PutChangelist(ctx, cl); err != nil {
		sklog.Errorf("Updating %s CL with id %q to clstore: %s", system.ID, clID, err)
		return ingestion.ErrRetryable
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
				// Try again later
				return code_review.Patchset{}, ingestion.ErrRetryable
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
			// Try again later
			return code_review.Patchset{}, ingestion.ErrRetryable
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
