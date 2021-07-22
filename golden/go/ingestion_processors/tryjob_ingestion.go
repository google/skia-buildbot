package ingestion_processors

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"go.skia.org/infra/golden/go/clstore"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/code_review/gerrit_crs"
	"go.skia.org/infra/golden/go/code_review/github_crs"
	"go.skia.org/infra/golden/go/continuous_integration"
	"go.skia.org/infra/golden/go/continuous_integration/buildbucket_cis"
	"go.skia.org/infra/golden/go/continuous_integration/simple_cis"
	"go.skia.org/infra/golden/go/ingestion"
	"go.skia.org/infra/golden/go/jsonio"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
)

const (
	SQLSecondaryBranch = "sql_secondary"

	codeReviewSystemsParam     = "CodeReviewSystems"
	gerritURLParam             = "GerritURL"
	gerritInternalURLParam     = "GerritInternalURL"
	githubRepoParam            = "GitHubRepo"
	githubCredentialsPathParam = "GitHubCredentialsPath"

	continuousIntegrationSystemsParam = "ContinuousIntegrationSystems"

	lookupParam = "LookupCLsIn"

	gerritCRS              = "gerrit"
	gerritInternalCRS      = "gerrit-internal"
	githubCRS              = "github"
	buildbucketCIS         = "buildbucket"
	buildbucketInternalCIS = "buildbucket-internal"
	cirrusCIS              = "cirrus"

	clCacheSize = 1000
)

// goldTryjobProcessor implements the ingestion.Processor interface to ingest tryjob results.
type goldTryjobProcessor struct {
	cisClients    map[string]continuous_integration.Client
	reviewSystems []clstore.ReviewSystem
	lookupSystem  LookupSystem
	source        ingestion.Source
	db            *pgxpool.Pool

	clCache             *lru.Cache
	optionGroupingCache *lru.Cache
	paramsCache         *lru.Cache
	traceCache          *lru.Cache
}

// TryjobSQL returns an ingestion.Processor which is modular and can support
// different CodeReviewSystems (e.g. "Gerrit", "GitHub") and different ContinuousIntegrationSystems
// (e.g. "BuildBucket", "CirrusCI"). This particular implementation stores the data in SQL.
func TryjobSQL(ctx context.Context, src ingestion.Source, configParams map[string]string, client *http.Client, db *pgxpool.Pool) (ingestion.Processor, error) {
	cisNames := strings.Split(configParams[continuousIntegrationSystemsParam], ",")
	if len(cisNames) == 0 {
		return nil, skerr.Fmt("missing CI system (e.g. 'buildbucket')")
	}
	cisClients := make(map[string]continuous_integration.Client, len(cisNames))
	for _, cisName := range cisNames {
		cis, err := continuousIntegrationSystemFactory(cisName, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create client for CIS %q", cisName)
		}
		cisClients[cisName] = cis
	}

	crsNames := strings.Split(configParams[codeReviewSystemsParam], ",")
	if len(crsNames) == 0 {
		return nil, skerr.Fmt("missing CRS (e.g. 'gerrit')")
	}

	var reviewSystems []clstore.ReviewSystem
	for _, crsName := range crsNames {
		crsClient, err := codeReviewSystemFactory(ctx, crsName, configParams, client)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not create client for CRS %q", crsName)
		}
		reviewSystems = append(reviewSystems, clstore.ReviewSystem{
			ID:     crsName,
			Client: crsClient,
		})
	}

	var lookupSystem LookupSystem
	if ls, ok := configParams[lookupParam]; ok {
		if ls != buildbucketCIS {
			return nil, skerr.Fmt("unknown lookup system: %s", ls)
		}
		bbClient := buildbucket.NewClient(client)
		lookupSystem = newBuildbucketLookupClient(bbClient)
	}

	ogCache, err := lru.New(optionsGroupingCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	paramsCache, err := lru.New(paramsCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	tCache, err := lru.New(traceCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}
	clCache, err := lru.New(clCacheSize)
	if err != nil {
		panic(err) // should only throw error on invalid size
	}

	return &goldTryjobProcessor{
		cisClients:    cisClients,
		reviewSystems: reviewSystems,
		lookupSystem:  lookupSystem,
		source:        src,

		db:                  db,
		clCache:             clCache,
		optionGroupingCache: ogCache,
		paramsCache:         paramsCache,
		traceCache:          tCache,
	}, nil
}

// HandlesFile returns true if the configured source handles this file.
func (g *goldTryjobProcessor) HandlesFile(name string) bool {
	return g.source.HandlesFile(name)
}

func codeReviewSystemFactory(ctx context.Context, crsName string, configParams map[string]string, client *http.Client) (code_review.Client, error) {
	if crsName == gerritCRS {
		gerritURL := configParams[gerritURLParam]
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
		// TODO(skbug.com/12007)
		sklog.Infof("Using rubberstamp CRS implementation for gerrit-internal")
		return rubberstampCRS{}, nil
	}
	if crsName == githubCRS {
		githubRepo := configParams[githubRepoParam]
		if strings.TrimSpace(githubRepo) == "" {
			return nil, skerr.Fmt("missing repo for the GitHub code review system")
		}
		githubCredPath := configParams[githubCredentialsPathParam]
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

func continuousIntegrationSystemFactory(cisName string, client *http.Client) (continuous_integration.Client, error) {
	if cisName == buildbucketCIS {
		bbClient := buildbucket.NewClient(client)
		return buildbucket_cis.New(bbClient), nil
	}
	if cisName == cirrusCIS {
		return simple_cis.New(cisName), nil
	}
	if cisName == buildbucketInternalCIS {
		// TODO(skbug.com/12011)
		return simple_cis.New(cisName), nil
	}
	return nil, skerr.Fmt("ContinuousIntegrationSystem %q not recognized", cisName)
}

// Process take the tryjob data from the given file and writes it to the various SQL tables
// required by the schema.
// If there is a SQL error, we return ingestion.ErrRetryable but do NOT rollback the data. This
// is the same strategy and rationale as ingesting on the primary branch.
func (g *goldTryjobProcessor) Process(ctx context.Context, fileName string) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
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
	if len(gr.Results) == 0 {
		sklog.Infof("file %s had no tryjob results", fileName)
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_results", int64(len(gr.Results))))
	sklog.Infof("Ingesting %d tryjob results from file %s", len(gr.Results), fileName)

	clID, psID, err := g.lookupCLAndPS(ctx, gr)
	if err != nil {
		return skerr.Wrapf(err, "Deriving CL and PS info for file %s", fileName)
	}

	tjID, err := g.lookupTryjob(ctx, gr, clID, psID)
	if err != nil {
		return skerr.Wrapf(err, "Deriving Tryjob info for file %s", fileName)
	}

	sourceFileID := md5.Sum([]byte(fileName))
	if err := g.writeData(ctx, gr, clID, psID, tjID, sourceFileID[:]); err != nil {
		sklog.Errorf("Error data for tryjob file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}

	// Write the same timestamp to all 3 of these tables.
	ingestedTime := now.Now(ctx)
	if err := g.upsertSourceFile(ctx, sourceFileID[:], fileName, ingestedTime); err != nil {
		sklog.Errorf("Error writing to SourceFiles for tryjob file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}
	if err := g.updateTryjob(ctx, tjID, ingestedTime); err != nil {
		sklog.Errorf("Error writing updated Tryjob time for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}
	if err := g.updateCL(ctx, clID, ingestedTime); err != nil {
		sklog.Errorf("Error writing updated CL time for file %s: %s", fileName, err)
		return ingestion.ErrRetryable
	}
	return nil
}

// lookupCLAndPS returns the qualified Changelist ID and Patchset ID for these given results if it
// was able to derive them. It will create entries in the DB for them if they do not exist, after
// looking them up with the code_review.Client if necessary.
func (g *goldTryjobProcessor) lookupCLAndPS(ctx context.Context, gr *jsonio.GoldResults) (string, string, error) {
	ctx, span := trace.StartSpan(ctx, "lookupCLAndPS")
	defer span.End()
	system, clID, psID, psOrder, err := g.getIDs(ctx, gr)
	if err != nil {
		return "", "", skerr.Wrap(err)
	}

	qualifiedCLID := sql.Qualify(system.ID, clID)

	row := g.db.QueryRow(ctx, `SELECT count(*) FROM Changelists
WHERE changelist_id = $1 AND system = $2`, qualifiedCLID, system.ID)
	count := -1
	if err := row.Scan(&count); err != nil {
		sklog.Errorf("Error fetching CL %s: %s", qualifiedCLID, err)
		return "", "", ingestion.ErrRetryable
	}
	if count == 0 {
		// Look it up and store it if it exists.
		if err := g.lookupAndCreateCL(ctx, system.Client, clID, system.ID); err != nil {
			return "", "", skerr.Wrapf(err, "problem initializing CL %s", clID)
		}
	}
	// We need to look up the PS ID to make sure it exists. Also, if the client gave us
	row = g.db.QueryRow(ctx, `SELECT patchset_id FROM Patchsets
WHERE changelist_id = $1 AND system = $2 AND (patchset_id = $3 OR ps_order = $4)`,
		qualifiedCLID, system.ID, sql.Qualify(system.ID, psID), psOrder)
	qualifiedPSID := ""
	if err := row.Scan(&qualifiedPSID); err != nil && err != pgx.ErrNoRows {
		sklog.Errorf("Error fetching PS %s, %d for CL: %s and CRS", psID, psOrder, clID, system.ID, err)
		return "", "", ingestion.ErrRetryable
	}
	if qualifiedPSID == "" {
		// Look it up and store it if it exists.
		qualifiedPSID, err = g.lookupAndCreatePS(ctx, system.Client, clID, psID, system.ID, psOrder)
		if err != nil {
			return "", "", skerr.Wrapf(err, "problem initializing PS %s-%s", clID, psID)
		}
	}
	return qualifiedCLID, qualifiedPSID, nil
}

// getIDs returns the ReviewSystem, CL ID, PS ID, and PS Order from the given results. This could
// result in looking that up using the lookupSystem (e.g. buildbucket).
func (g *goldTryjobProcessor) getIDs(ctx context.Context, gr *jsonio.GoldResults) (clstore.ReviewSystem, string, string, int, error) {
	ctx, span := trace.StartSpan(ctx, "getIDs")
	defer span.End()
	crsName := gr.CodeReviewSystem
	if crsName == "" {
		// Default to Gerrit; TODO(kjlubick) who uses this?
		sklog.Warningf("Using default CRS (this may go away soon)")
		crsName = gerritCRS
	}
	if crsName != "lookup" {
		system, ok := g.getCodeReviewSystem(crsName)
		if !ok {
			return clstore.ReviewSystem{}, "", "", 0, skerr.Fmt("unsupported CRS: %s", crsName)
		}
		return system, gr.ChangelistID, gr.PatchsetID, gr.PatchsetOrder, nil
	}
	if g.lookupSystem == nil {
		return clstore.ReviewSystem{}, "", "", 0, skerr.Fmt("Lookup of CL/PS is not configured")
	}

	if val, ok := g.clCache.Get(gr.TryJobID); ok {
		ce, ok := val.(lookupCacheEntry)
		if ok {
			system, ok := g.getCodeReviewSystem(ce.crsName)
			if !ok {
				return clstore.ReviewSystem{}, "", "", 0, skerr.Fmt("unsupported CRS after lookup: %s", ce.crsName)
			}
			return system, ce.clID, "", ce.psOrder, nil
		}
	}
	crsName, clID, psOrder, err := g.lookupSystem.Lookup(ctx, gr.TryJobID)
	if err != nil {
		return clstore.ReviewSystem{}, "", "", 0, skerr.Wrapf(err, "lookup up CL and PS from buildbucket %s", gr.TryJobID)
	}
	system, ok := g.getCodeReviewSystem(crsName)
	if !ok {
		return clstore.ReviewSystem{}, "", "", 0, skerr.Fmt("unsupported CRS after lookup: %s", crsName)
	}
	g.clCache.Add(gr.TryJobID, lookupCacheEntry{
		crsName: crsName,
		clID:    clID,
		psOrder: psOrder,
	})
	return system, clID, "", psOrder, nil
}

type lookupCacheEntry struct {
	crsName string
	clID    string
	psOrder int
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

// lookupAndCreateCL finds the changelist with the given id and creates an entry in the SQL DB
// if it is found.
func (g *goldTryjobProcessor) lookupAndCreateCL(ctx context.Context, client code_review.Client, id, crs string) error {
	ctx, span := trace.StartSpan(ctx, "lookupAndCreateCL")
	defer span.End()
	cl, err := client.GetChangelist(ctx, id)
	if err != nil {
		return skerr.Wrap(err)
	}
	qID := sql.Qualify(crs, id)
	const statement = `
INSERT INTO Changelists (changelist_id, system, status, owner_email, subject, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING`
	err = crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, qID, crs, convertFromStatusEnum(cl.Status), cl.Owner, cl.Subject, cl.Updated)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		sklog.Errorf("Error Inserting CL %#v: %s", cl, err)
		return ingestion.ErrRetryable
	}
	return nil
}

// lookupAndCreatePS finds the patchset with the given id or order belonging to the provided CL.
// It creates an entry in the SQL db and returns the qualified Patchset ID if successful.
func (g *goldTryjobProcessor) lookupAndCreatePS(ctx context.Context, client code_review.Client, clID, psID, crs string, psOrder int) (string, error) {
	ctx, span := trace.StartSpan(ctx, "lookupAndCreatePS")
	defer span.End()

	ps, err := client.GetPatchset(ctx, clID, psID, psOrder)
	if err != nil {
		return "", skerr.Wrap(err)
	}

	qualifiedCLID := sql.Qualify(crs, ps.ChangelistID)
	qualifiedPSID := sql.Qualify(crs, ps.SystemID)
	const statement = `
INSERT INTO Patchsets (patchset_id, system, changelist_id, ps_order, git_hash,
  commented_on_cl, created_ts)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT DO NOTHING`
	err = crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, qualifiedPSID, crs, qualifiedCLID, ps.Order, ps.GitHash,
			false, ps.Created)
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		sklog.Errorf("Error inserting patchset %#v: %s", ps, err)
		return "", ingestion.ErrRetryable
	}
	return qualifiedPSID, nil
}

// lookupTryjob returns the qualified Tryjob ID for these given results if derivation was
// successful. It will create an entry in the DB if it does not exist, using the ci.Client
// if necessary to look it up. The created entry will be related to the provided CL and PS.
func (g *goldTryjobProcessor) lookupTryjob(ctx context.Context, gr *jsonio.GoldResults, clID, psID string) (string, error) {
	ctx, span := trace.StartSpan(ctx, "lookupTryjob")
	defer span.End()
	cisName := gr.ContinuousIntegrationSystem
	if cisName == "" {
		// Default to BuildBucket; TODO(kjlubick) who uses this?
		sklog.Warningf("Using default CIS (this may go away soon)")
		cisName = buildbucketCIS
	}
	system, ok := g.cisClients[cisName]
	if !ok {
		return "", skerr.Fmt("unsupported CIS: %s", cisName)
	}

	tjID := gr.TryJobID
	qualifiedTJID := sql.Qualify(cisName, tjID)

	row := g.db.QueryRow(ctx, `SELECT count(*) FROM Tryjobs
WHERE tryjob_id = $1 AND system = $2`, qualifiedTJID, cisName)
	count := -1
	if err := row.Scan(&count); err != nil {
		sklog.Errorf("Error fetching TJ %s: %s", qualifiedTJID, err)
		return "", ingestion.ErrRetryable
	}
	if count == 0 {
		// Look it up and store it if it exists.
		if err := g.lookupAndCreateTryjob(ctx, system, clID, psID, tjID); err != nil {
			return "", skerr.Wrapf(err, "problem initializing Tryjob %s for CL %s-%s", tjID, clID, psID)
		}
	}
	return qualifiedTJID, nil
}

// lookupAndCreateCL finds the changelist with the given id and creates an entry in the SQL DB
// if it is found. Note that there must exist entries for the CL and PS that are passed in, due to
// the foreign key constraints.
func (g *goldTryjobProcessor) lookupAndCreateTryjob(ctx context.Context, client continuous_integration.Client, clID, psID, tjID string) error {
	ctx, span := trace.StartSpan(ctx, "lookupAndCreateTryjob")
	defer span.End()

	tj, err := client.GetTryJob(ctx, tjID)
	if err != nil {
		return skerr.Wrapf(err, "looking up Tryjob %s", tjID)
	}

	qID := sql.Qualify(tj.System, tj.SystemID)
	const statement = `
UPSERT INTO Tryjobs (tryjob_id, system, changelist_id, patchset_id, display_name, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)`
	err = crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, qID, tj.System, clID, psID, tj.DisplayName, time.Time{})
		return err // Don't wrap - crdbpgx might retry
	})
	if err != nil {
		return skerr.Wrapf(err, "Inserting tryjob %#v", tj)
	}
	return nil
}

// writeData writes all the data from the processed JSON file, associating it with the given
// commitID, tileID, and sourceFile. This has to write to several tables in accordance with the
// schema/design. It makes use of caches where possible to avoid writing to tables with immutable
// data that we know is there already (e.g. a previous write succeeded).
func (g *goldTryjobProcessor) writeData(ctx context.Context, gr *jsonio.GoldResults, clID, psID, tjID string, srcID schema.SourceFileID) error {
	ctx, span := trace.StartSpan(ctx, "writeData")
	span.AddAttributes(trace.Int64Attribute("results", int64(len(gr.Results))))
	defer span.End()

	var groupingsToCreate []schema.GroupingRow
	var optionsToCreate []schema.OptionsRow
	var tracesToCreate []schema.TraceRow
	var traceValuesToUpdate []schema.SecondaryBranchValueRow

	newCacheEntries := map[string]bool{}
	paramset := paramtools.ParamSet{} // All params for this set of data points
	for _, result := range gr.Results {
		keys, options := paramsAndOptions(gr, result)
		if err := shouldIngest(keys, options); err != nil {
			sklog.Infof("Not ingesting a result: %s", err)
			continue
		}
		digestBytes, err := sql.DigestToBytes(result.Digest)
		if err != nil {
			sklog.Errorf("Invalid digest %s: %s", result.Digest, err)
			continue
		}
		_, traceID := sql.SerializeMap(keys)
		paramset.AddParams(keys)

		_, optionsID := sql.SerializeMap(options)
		// We explicitly do not add options to paramset, but may store them to a different
		// table in the future.

		grouping := groupingFor(keys)
		_, groupingID := sql.SerializeMap(grouping)

		if h := string(optionsID); !g.optionGroupingCache.Contains(h) && !newCacheEntries[h] {
			optionsToCreate = append(optionsToCreate, schema.OptionsRow{
				OptionsID: optionsID,
				Keys:      options,
			})
			newCacheEntries[h] = true
		}

		if h := string(groupingID); !g.optionGroupingCache.Contains(h) && !newCacheEntries[h] {
			groupingsToCreate = append(groupingsToCreate, schema.GroupingRow{
				GroupingID: groupingID,
				Keys:       grouping,
			})
			newCacheEntries[h] = true
		}

		th := string(traceID)
		if newCacheEntries[th] {
			continue // already seen data for this trace
		}
		newCacheEntries[th] = true
		if !g.traceCache.Contains(th) {
			tracesToCreate = append(tracesToCreate, schema.TraceRow{
				TraceID:              traceID,
				GroupingID:           groupingID,
				Keys:                 keys,
				MatchesAnyIgnoreRule: schema.NBNull,
			})
		}
		traceValuesToUpdate = append(traceValuesToUpdate, schema.SecondaryBranchValueRow{
			BranchName:   clID,
			VersionName:  psID,
			TraceID:      traceID,
			Digest:       digestBytes,
			GroupingID:   groupingID,
			OptionsID:    optionsID,
			SourceFileID: srcID,
			TryjobID:     tjID,
		})
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		return skerr.Wrap(batchCreateGroupings(ctx, g.db, groupingsToCreate, g.optionGroupingCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(batchCreateOptions(ctx, g.db, optionsToCreate, g.optionGroupingCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(batchCreateTraces(ctx, g.db, tracesToCreate, g.traceCache))
	})
	eg.Go(func() error {
		return skerr.Wrap(g.batchUpdateSecondaryBranchValues(ctx, traceValuesToUpdate))
	})
	eg.Go(func() error {
		return skerr.Wrap(g.batchCreateSecondaryBranchParams(ctx, paramset, clID, psID))
	})
	return skerr.Wrap(eg.Wait())
}

// batchUpdateSecondaryBranchValues writes the given data points to the DB.
func (g *goldTryjobProcessor) batchUpdateSecondaryBranchValues(ctx context.Context, rows []schema.SecondaryBranchValueRow) error {
	ctx, span := trace.StartSpan(ctx, "batchUpdateSecondaryBranchValues")
	defer span.End()
	if len(rows) == 0 {
		return nil
	}
	// Start at this chunk size for now. This table will likely receive a fair amount of data
	// and smaller batch sizes can reduce the contention/retries.
	const chunkSize = 300
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `UPSERT INTO SecondaryBranchValues
(branch_name, version_name, secondary_branch_trace_id, digest, grouping_id, options_id,
source_file_id, tryjob_id) VALUES `
		const valuesPerRow = 8
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, value := range batch {
			arguments = append(arguments, value.BranchName, value.VersionName, value.TraceID,
				value.Digest, value.GroupingID, value.OptionsID, value.SourceFileID, value.TryjobID)
		}

		err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d SecondaryBranchValues", len(rows))
	}
	return nil
}

// batchCreateSecondaryBranchParams writes the provided key-value pairs to the DB associated with
// the provided PS and CL. It updates the cache on a successfull write.
func (g *goldTryjobProcessor) batchCreateSecondaryBranchParams(ctx context.Context, paramset paramtools.ParamSet, clID string, psID string) error {
	ctx, span := trace.StartSpan(ctx, "batchCreateSecondaryBranchParams")
	defer span.End()
	var rows []schema.SecondaryBranchParamRow
	for key, values := range paramset {
		for _, value := range values {
			pr := schema.SecondaryBranchParamRow{
				BranchName:  clID,
				VersionName: psID,
				Key:         key,
				Value:       value,
			}
			if g.paramsCache.Contains(pr) {
				continue // don't need to store it again.
			}
			rows = append(rows, pr)
		}
	}

	if len(rows) == 0 {
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("key_value_pairs", int64(len(rows))))

	const chunkSize = 200 // Arbitrarily picked
	err := util.ChunkIter(len(rows), chunkSize, func(startIdx int, endIdx int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := rows[startIdx:endIdx]
		if len(batch) == 0 {
			return nil
		}
		statement := `INSERT INTO SecondaryBranchParams (branch_name, version_name, key, value) VALUES `
		const valuesPerRow = 4
		statement += sql.ValuesPlaceholders(valuesPerRow, len(batch))
		arguments := make([]interface{}, 0, valuesPerRow*len(batch))
		for _, row := range batch {
			arguments = append(arguments, row.BranchName, row.VersionName, row.Key, row.Value)
		}
		// ON CONFLICT DO NOTHING because if the rows already exist, the data is immutable.
		statement += ` ON CONFLICT DO NOTHING;`

		err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, statement, arguments...)
			return err // Don't wrap - crdbpgx might retry
		})
		return skerr.Wrap(err)
	})
	if err != nil {
		return skerr.Wrapf(err, "storing %d secondary branch params", len(rows))
	}

	for _, r := range rows {
		g.paramsCache.Add(r, struct{}{})
	}
	return nil
}

// upsertSourceFile creates a row in SourceFiles for the given file or updates the existing row's
// last_ingested timestamp with the provided time.
func (g *goldTryjobProcessor) upsertSourceFile(ctx context.Context, srcID schema.SourceFileID, fileName string, ingestedTime time.Time) interface{} {
	ctx, span := trace.StartSpan(ctx, "upsertSourceFile")
	defer span.End()
	const statement = `UPSERT INTO SourceFiles (source_file_id, source_file, last_ingested)
VALUES ($1, $2, $3)`
	err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, srcID, fileName, ingestedTime)
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrap(err)
}

// updateCL updates the last_ingested_data timestamp for this Tryjob.
func (g *goldTryjobProcessor) updateTryjob(ctx context.Context, id string, ts time.Time) error {
	ctx, span := trace.StartSpan(ctx, "updateTryjob")
	defer span.End()
	const statement = `UPDATE Tryjobs SET last_ingested_data = $1
WHERE tryjob_id = $2`
	err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, ts, id)
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrapf(err, "updating time on Tryjob %s", id)
}

// updateCL updates the last_ingested_data timestamp for this CL.
func (g *goldTryjobProcessor) updateCL(ctx context.Context, id string, ts time.Time) error {
	ctx, span := trace.StartSpan(ctx, "updateCL")
	defer span.End()
	const statement = `UPDATE Changelists SET last_ingested_data = $1
WHERE changelist_id = $2`
	err := crdbpgx.ExecuteTx(ctx, g.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, statement, ts, id)
		return err // Don't wrap - crdbpgx might retry
	})
	return skerr.Wrapf(err, "updating time on CL %s", id)
}

// convertFromStatusEnum returns a SQL version of the CLStatus.
func convertFromStatusEnum(status code_review.CLStatus) schema.ChangelistStatus {
	switch status {
	case code_review.Abandoned:
		return schema.StatusAbandoned
	case code_review.Open:
		return schema.StatusOpen
	case code_review.Landed:
		return schema.StatusLanded
	}
	sklog.Warningf("Unknown status: %d", status)
	return schema.StatusAbandoned
}

// Make sure goldTryjobProcessor implements the ingestion.Processor interface.
var _ ingestion.Processor = (*goldTryjobProcessor)(nil)

// rubberstampCRS implements a simple Code Review System that pretends every CL it sees exists.
type rubberstampCRS struct {
}

func (r rubberstampCRS) GetChangelist(_ context.Context, id string) (code_review.Changelist, error) {
	sklog.Infof("Rubberstamp CL response for %s", id)
	return code_review.Changelist{
		SystemID: id,
		Owner:    "<unknown>",
		Status:   code_review.Open,
		Subject:  "<unknown>",
		Updated:  time.Now(),
	}, nil
}

func (r rubberstampCRS) GetPatchset(_ context.Context, clID, psID string, psOrder int) (code_review.Patchset, error) {
	if psOrder == 0 {
		return code_review.Patchset{}, skerr.Fmt("The order of the Patchset must be provided in rubberstamp mode")
	}
	sklog.Infof("Rubberstamp PS response for %s %s %d", clID, psID, psOrder)
	return code_review.Patchset{
		SystemID:     fmt.Sprintf("%s|%s|%d", clID, psID, psOrder),
		ChangelistID: clID,
		Order:        psOrder,
		GitHash:      "<unknown>",
	}, nil
}

func (r rubberstampCRS) GetChangelistIDForCommit(_ context.Context, _ *vcsinfo.LongCommit) (string, error) {
	return "", skerr.Fmt("not implemented")
}

func (r rubberstampCRS) CommentOn(_ context.Context, _, _ string) error {
	return skerr.Fmt("not implemented")
}
