// Package commenter contains an implementation of the code_review.ChangelistCommenter interface.
// It should be CRS-agnostic.
package commenter

import (
	"bytes"
	"context"
	"text/template"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/code_review"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
)

const (
	numRecentOpenCLsMetric = "gold_num_recent_open_cls"
)

type ReviewSystem struct {
	ID     string // e.g. "gerrit", "gerrit-internal"
	Client code_review.Client
}

type Impl struct {
	db              *pgxpool.Pool
	instanceURL     string
	messageTemplate *template.Template
	systems         []ReviewSystem
	lastCheck       time.Time
	commitsInWindow int
}

func New(db *pgxpool.Pool, systems []ReviewSystem, messageTemplate, instanceURL string, windowSize int) (*Impl, error) {
	templ, err := template.New("message").Parse(messageTemplate)
	if err != nil && messageTemplate != "" {
		return nil, skerr.Wrapf(err, "Message template %q", messageTemplate)
	}
	return &Impl{
		db:              db,
		instanceURL:     instanceURL,
		messageTemplate: templ,
		systems:         systems,
		commitsInWindow: windowSize,
	}, nil
}

// CommentOnChangelistsWithUntriagedDigests implements the code_review.ChangelistCommenter
// interface.
func (i *Impl) CommentOnChangelistsWithUntriagedDigests(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "commenter_CommentOnChangelistsWithUntriagedDigests")
	defer span.End()
	if i.lastCheck.IsZero() {
		// Default to checking all CLs that had data ingested within the last day.
		i.lastCheck = now.Now(ctx).Add(-1 * 24 * time.Hour)
	}
	lastCheckUpdate := now.Now(ctx)
	patchsets, err := i.getNewestPatchsets(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(patchsets) == 0 {
		i.lastCheck = lastCheckUpdate
		sklog.Infof("No patchsets had seen updated since last check.")
		return nil
	}
	err = i.addNewDigestCounts(ctx, patchsets)
	if err != nil {
		return skerr.Wrap(err)
	}
	for _, ps := range patchsets {
		if ps.numNewDigests > 0 {
			if err := i.commentOn(ctx, *ps); err != nil {
				sklog.Warningf("Could not comment on CL and PS %#v: %s", ps, err)
				// Continue anyway - don't let one problematic CL stop the rest.
			}
		}
	}
	// actually check them for untriaged digests
	i.lastCheck = lastCheckUpdate
	return nil
}

type patchsetInfo struct {
	system        string
	changelistID  string // qualified id
	patchsetID    string // qualified id
	order         int
	numNewDigests int // an approximate count
}

// getNewestPatchsets returns the newest patchset for each open CL that had new data since the
// last time we checked. It filters out any patchsets that we've already commented on.
func (i *Impl) getNewestPatchsets(ctx context.Context) ([]*patchsetInfo, error) {
	ctx, span := trace.StartSpan(ctx, "getNewestPatchsets")
	defer span.End()
	// Select the most recent patchset with data from all changelists that had newly triaged data
	// since the last time we checked.
	const statement = `WITH
ChangelistsWithNewData AS (
	SELECT changelist_id FROM Changelists
	WHERE status = 'open' and last_ingested_data > $1
)
SELECT DISTINCT ON (system, changelist_id)
	Patchsets.system, Patchsets.changelist_id, patchset_id, ps_order, commented_on_cl FROM Patchsets
  JOIN ChangelistsWithNewData on Patchsets.changelist_id = ChangelistsWithNewData.changelist_id
ORDER BY system, changelist_id, ps_order DESC
`
	rows, err := i.db.Query(ctx, statement, i.lastCheck)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []*patchsetInfo
	openCLs := 0
	for rows.Next() {
		var row patchsetInfo
		var commentAlready bool
		if err := rows.Scan(&row.system, &row.changelistID, &row.patchsetID, &row.order, &commentAlready); err != nil {
			return nil, skerr.Wrap(err)
		}
		openCLs++
		if commentAlready {
			// We don't bother with CLs for which we have already commented on the most recent PS.
			continue
		}
		rv = append(rv, &row)
	}
	metrics2.GetInt64Metric(numRecentOpenCLsMetric, nil).Update(int64(openCLs))
	return rv, nil
}

// addNewDigestCounts counts how many new images were produced for each of the patchsets. "New"
// means not seen in the current commit window.
func (i *Impl) addNewDigestCounts(ctx context.Context, patchsets []*patchsetInfo) error {
	ctx, span := trace.StartSpan(ctx, "addNewDigestCounts")
	defer span.End()
	// We get the digests as their own step because we need to look up our secondary branch values
	// using pairs of (branch_name, version_name), which is a bit awkward to write in pure SQL.
	digestsOnPrimary, err := i.getDigestsOnPrimary(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}
	eg, eCtx := errgroup.WithContext(ctx)
	for idx := range patchsets {
		ps := patchsets[idx]
		eg.Go(func() error {
			const statement = `SELECT DISTINCT digest
FROM SecondaryBranchValues WHERE branch_name = $1 AND version_name = $2`
			rows, err := i.db.Query(eCtx, statement, ps.changelistID, ps.patchsetID)
			if err != nil {
				return skerr.Wrapf(err, "patchset %#v", *ps)
			}
			defer rows.Close()
			newDigests := 0
			var digestBytes schema.DigestBytes
			var digestKey schema.MD5Hash
			digest := digestKey[:]
			for rows.Next() {
				if err := rows.Scan(&digestBytes); err != nil {
					return skerr.Wrap(err)
				}
				copy(digest, digestBytes)
				if _, ok := digestsOnPrimary[digestKey]; !ok {
					newDigests++
				}
			}
			ps.numNewDigests = newDigests
			return nil
		})
	}
	return skerr.Wrap(eg.Wait())
}

// getDigestsOnPrimary returns a set of all digests currently on the primary branch.
func (i *Impl) getDigestsOnPrimary(ctx context.Context) (map[schema.MD5Hash]struct{}, error) {
	ctx, span := trace.StartSpan(ctx, "getDigestsOnPrimary")
	defer span.End()
	const statement = `WITH
BeginningOfWindow AS (
    SELECT tile_id FROM (
		SELECT commit_id, tile_id FROM CommitsWithData
		ORDER BY commit_id DESC
		LIMIT $1
	) ORDER BY commit_id ASC LIMIT 1
)
SELECT DISTINCT digest FROM TiledTraceDigests
	JOIN BeginningOfWindow ON TiledTraceDigests.tile_id >= BeginningOfWindow.tile_id
`
	rows, err := i.db.Query(ctx, statement, i.commitsInWindow)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	rv := map[schema.MD5Hash]struct{}{}
	var seen struct{}
	var digestBytes schema.DigestBytes
	var digestKey schema.MD5Hash
	digest := digestKey[:]
	for rows.Next() {
		if err := rows.Scan(&digestBytes); err != nil {
			return nil, skerr.Wrap(err)
		}
		copy(digest, digestBytes)
		rv[digestKey] = seen
	}
	return rv, nil
}

// commentOn either comments on the given CL/PS that there are untriaged digests on it or
// logs if this commenter is configured to not actually comment.
func (i *Impl) commentOn(ctx context.Context, ps patchsetInfo) error {
	clID := sql.Unqualify(ps.changelistID)
	msg, err := i.untriagedMessage(commentTemplateContext{
		CRS:           ps.system,
		ChangelistID:  clID,
		PatchsetOrder: ps.order,
		NumNewDigests: ps.numNewDigests,
	})
	if err != nil {
		return skerr.Wrap(err)
	}
	var client code_review.Client
	for _, c := range i.systems {
		if c.ID == ps.system {
			client = c.Client
		}
	}
	if client == nil {
		sklog.Errorf("Could not make comment for system %s - not configured", ps.system)
		return nil
	}
	if cl, err := client.GetChangelist(ctx, clID); err != nil {
		if err == code_review.ErrNotFound {
			sklog.Infof("CL %s might have been deleted", clID)
			return nil
		}
		return skerr.Wrap(err)
	} else {
		if cl.Status != code_review.Open {
			sklog.Infof("CL %s was not open - %v", clID, cl.Status)
			return nil
		}
	}

	sklog.Infof("Commenting on CL %s PS %d about newly produced images", clID, ps.order)
	if err := client.CommentOn(ctx, clID, msg); err != nil {
		return skerr.Wrapf(err, "commenting on %s CL %s", ps.system, clID)
	}
	const statement = `UPDATE Patchsets SET commented_on_cl = TRUE WHERE patchset_id = $1`
	_, err = i.db.Exec(ctx, statement, ps.patchsetID)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// commentTemplateContext contains the fields that can be substituted into
type commentTemplateContext struct {
	ChangelistID  string
	CRS           string
	InstanceURL   string
	NumNewDigests int
	PatchsetOrder int
}

// untriagedMessage returns a message about untriaged images on the given CL/PS.
func (i *Impl) untriagedMessage(c commentTemplateContext) (string, error) {
	c.InstanceURL = i.instanceURL
	var b bytes.Buffer
	if err := i.messageTemplate.Execute(&b, c); err != nil {
		return "", skerr.Wrapf(err, "With template context %#v", c)
	}
	return b.String(), nil
}

// Make sure Impl fulfills the code_review.ChangelistCommenter interface.
var _ code_review.ChangelistCommenter = (*Impl)(nil)
