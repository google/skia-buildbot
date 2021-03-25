package ingestion_processors

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/golden/go/sql/schema"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/jackc/pgx/v4"

	"go.skia.org/infra/golden/go/sql"

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
	"go.skia.org/infra/golden/go/tjstore"
	"go.skia.org/infra/golden/go/tjstore/sqltjstore"
)

const (
	SQLSecondaryBranch = "sql_secondary"

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
	db            *pgxpool.Pool
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
		gerritURL := configParams[gerritInternalURLParam]
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

	ingestedTime := now(ctx)
	if err := g.upsertSourceFile(ctx, sourceFileID[:], fileName, ingestedTime); err != nil {
		sklog.Errorf("Error writing to SourceFiles for tryjob file %s: %s", fileName, err)
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
	crsName := gr.CodeReviewSystem
	if crsName == "" {
		// Default to Gerrit; TODO(kjlubick) who uses this?
		sklog.Warningf("Using default CRS (this may go away soon)")
		crsName = gerritCRS
	}
	system, ok := g.getCodeReviewSystem(crsName)
	if !ok {
		return "", "", skerr.Fmt("unsupported CRS: %s", crsName)
	}
	clID := gr.ChangelistID
	psOrder := gr.PatchsetOrder
	psID := gr.PatchsetID

	qualifiedCLID := sql.Qualify(system.ID, clID)
	qualifiedPSID := sql.Qualify(system.ID, psID)

	row := g.db.QueryRow(ctx, `SELECT count(*) FROM Changelists
WHERE changelist_id = $1 AND system = $2`, qualifiedCLID, system.ID)
	count := -1
	if err := row.Scan(&count); err != nil {
		sklog.Errorf("Error fetching CL %s", qualifiedCLID)
		return "", "", ingestion.ErrRetryable
	}
	if count == 0 {
		// Look it up and store it if it exists.
		if err := g.lookupAndCreateCL(ctx, system.Client, clID, system.ID); err != nil {
			return "", "", skerr.Wrapf(err, "problem initializing CL %s", clID)
		}
	}

	row = g.db.QueryRow(ctx, `SELECT count(*) FROM Patchsets
WHERE changelist_id = $1 AND system = $2 AND (patchset_id = $3 OR ps_order = $4)`,
		qualifiedCLID, system.ID, qualifiedPSID, psOrder)
	count = -1
	if err := row.Scan(&count); err != nil {
		sklog.Errorf("Error fetching PS %s, %d for CL", qualifiedPSID, psOrder, qualifiedCLID)
		return "", "", ingestion.ErrRetryable
	}
	if count == 0 {
		// TODO(kjlubick) look it up and store it
	}
	return qualifiedCLID, qualifiedPSID, nil
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

func (g *goldTryjobProcessor) lookupAndCreateCL(ctx context.Context, client code_review.Client, id, crs string) error {
	ctx, span := trace.StartSpan(ctx, "lookupAndCreateCL")
	defer span.End()
	cl, err := client.GetChangelist(ctx, id)
	if err != nil {
		return skerr.Wrap(err)
	}
	qID := sql.Qualify(crs, cl.SystemID)
	const statement = `
UPSERT INTO Changelists (changelist_id, system, status, owner_email, subject, last_ingested_data)
VALUES ($1, $2, $3, $4, $5, $6)`
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
		sklog.Errorf("Error fetching TJ %s", qualifiedTJID)
		return "", ingestion.ErrRetryable
	}
	if count == 0 {
		// TODO(kjlubick) look it up and store it
		fmt.Printf("%v", system)
	}

	return tjID, nil
}

func (g *goldTryjobProcessor) writeData(ctx context.Context, gr *jsonio.GoldResults, clID, psID, tjID string, srcID schema.SourceFileID) error {
	ctx, span := trace.StartSpan(ctx, "writeData")
	defer span.End()

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
