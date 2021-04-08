package sqlwrapped

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// Impl wraps a Firestore-backed Expectation Store with a layer to write changes to a SQL database
// but return everything from the Firestore DB. This will be in place until the existing search
// implementation can be replaced, due to the interwoven nature of the querysnapshots of the
// Firestore expectations and the current searching code.
type Impl struct {
	LegacyStore expectations.Store
	sqlDB       *pgxpool.Pool
	branch      string
}

func New(store expectations.Store, db *pgxpool.Pool) *Impl {
	return &Impl{
		LegacyStore: store,
		sqlDB:       db,
	}
}

// Get returns the result from the wrapped Store.
func (i *Impl) Get(ctx context.Context) (expectations.ReadOnly, error) {
	return i.LegacyStore.Get(ctx)
}

// GetCopy returns the result from the wrapped Store.
func (i *Impl) GetCopy(ctx context.Context) (*expectations.Expectations, error) {
	return i.LegacyStore.GetCopy(ctx)
}

// AddChange first adds the change to the Firestore database - if that succeeds, it writes the
// corresponding values to the SQL DB. Because the incoming deltas only have the test name, it
// needs to look up the associated corpora with those. It writes the expectations to the SQL db
// in one transaction, so to avoid partial commit errors.
func (i *Impl) AddChange(ctx context.Context, changes []expectations.Delta, userID string) error {
	ctx, span := trace.StartSpan(ctx, "sqlwrapped_AddChange", trace.WithSampler(trace.AlwaysSample()))
	span.AddAttributes(trace.Int64Attribute("num_total_changes", int64(len(changes))))
	defer span.End()
	if err := i.LegacyStore.AddChange(ctx, changes, userID); err != nil {
		return skerr.Wrap(err)
	}

	deltas, err := i.resolveGroupings(ctx, changes)
	if err != nil {
		return skerr.Wrapf(err, "getting groupings for %d changes", len(changes))
	}
	if len(deltas) == 0 {
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_changes", int64(len(deltas))))

	err = crdbpgx.ExecuteTx(ctx, i.sqlDB, pgx.TxOptions{}, func(tx pgx.Tx) error {
		expID, err := writeRecord(ctx, tx, userID, len(deltas), i.branch)
		if err != nil {
			return err
		}
		err = fillPreviousLabel(ctx, tx, deltas, expID)
		if err != nil {
			return err
		}
		err = writeDeltas(ctx, tx, deltas)
		if err != nil {
			return err
		}
		if i.branch == "" {
			return writeExpectations(ctx, tx, deltas)
		}
		return writeSecondaryExpectations(ctx, tx, deltas, i.branch)
	})
	if err != nil {
		return skerr.Wrapf(err, "writing %d expectations from %s", len(changes), userID)
	}
	return nil
}

// writeRecord writes a new ExpectationRecord to the DB.
func writeRecord(ctx context.Context, tx pgx.Tx, userID string, numChanges int, branch string) (uuid.UUID, error) {
	ctx, span := trace.StartSpan(ctx, "writeRecord")
	defer span.End()

	var br *string
	if branch != "" {
		br = &branch
	}
	const statement = `INSERT INTO ExpectationRecords
(user_name, triage_time, num_changes, branch_name) VALUES ($1, $2, $3, $4) RETURNING expectation_record_id`
	row := tx.QueryRow(ctx, statement, userID, now(ctx), numChanges, br)
	var recordUUID uuid.UUID
	err := row.Scan(&recordUUID)
	if err != nil {
		return uuid.UUID{}, skerr.Wrapf(err, "getting new UUID")
	}
	return recordUUID, nil
}

// resolveGroupings creates the initial ExpectationDeltaRows by looking up the groupings based on
// the test names. If a test name could belong to more than one grouping, it is undetermined which
// will be returned.
func (i *Impl) resolveGroupings(ctx context.Context, changes []expectations.Delta) ([]schema.ExpectationDeltaRow, error) {
	ctx, span := trace.StartSpan(ctx, "resolveGroupings")
	defer span.End()
	uniquetests := map[types.TestName]schema.GroupingID{}
	for _, c := range changes {
		uniquetests[c.Grouping] = nil
	}

	const statement = `
SELECT keys -> 'name', grouping_id FROM Groupings where keys -> 'name' IN `
	arguments := make([]interface{}, 0, len(uniquetests))
	for tn := range uniquetests {
		arguments = append(arguments, tn)
	}
	vp := sql.ValuesPlaceholders(len(uniquetests), 1)
	rows, err := i.sqlDB.Query(ctx, statement+vp, arguments...)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting groupings for %s", arguments)
	}
	defer rows.Close()
	for rows.Next() {
		var tn types.TestName
		var gID schema.GroupingID
		if err := rows.Scan(&tn, &gID); err != nil {
			return nil, skerr.Wrap(err)
		}
		uniquetests[tn] = gID
	}

	var rv []schema.ExpectationDeltaRow
	for _, c := range changes {
		d, err := sql.DigestToBytes(c.Digest)
		if err != nil {
			return nil, skerr.Wrapf(err, "invalid digest %q", c.Digest)
		}
		gID, ok := uniquetests[c.Grouping]
		if !ok || gID == nil {
			sklog.Warningf("Cannot find grouping for test %s", c.Grouping)
			continue
		}
		rv = append(rv, schema.ExpectationDeltaRow{
			GroupingID: gID,
			Digest:     d,
			LabelAfter: convertLabel(c.Label),
		})
	}
	return rv, nil
}

type expectationKey struct {
	groupingID schema.MD5Hash
	digest     schema.MD5Hash
}

// fillPreviousLabel looks up all the expectations for the partially filled-out deltas passed in
// and updates those in-place.
func fillPreviousLabel(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow, expID uuid.UUID) error {
	ctx, span := trace.StartSpan(ctx, "fillPreviousLabel")
	defer span.End()
	toUpdate := map[expectationKey]*schema.ExpectationDeltaRow{}
	for i := range deltas {
		deltas[i].ExpectationRecordID = expID         // fill in expectation record while we are here
		deltas[i].LabelBefore = schema.LabelUntriaged // default to untriaged
		toUpdate[expectationKey{
			groupingID: sql.AsMD5Hash(deltas[i].GroupingID),
			digest:     sql.AsMD5Hash(deltas[i].Digest),
		}] = &deltas[i]
	}

	statement := `SELECT grouping_id, digest, label FROM Expectations WHERE `
	// We should be safe from injection attacks because we are hex encoding known valid byte arrays.
	// I couldn't find a better way to match multiple composite keys using our usual techniques
	// involving placeholders.
	for i, d := range deltas {
		if i != 0 {
			statement += " OR "
		}
		statement += fmt.Sprintf(`(grouping_id = x'%x' AND digest = x'%x')`, d.GroupingID, d.Digest)
	}
	rows, err := tx.Query(ctx, statement)
	if err != nil {
		return err // don't wrap, could be retried
	}
	defer rows.Close()
	for rows.Next() {
		var gID schema.GroupingID
		var d schema.DigestBytes
		var label schema.ExpectationLabel
		if err := rows.Scan(&gID, &d, &label); err != nil {
			return skerr.Wrap(err) // probably not retryable
		}
		ek := expectationKey{
			groupingID: sql.AsMD5Hash(gID),
			digest:     sql.AsMD5Hash(d),
		}
		row := toUpdate[ek]
		if row == nil {
			sklog.Warningf("Unmatched row with grouping %x and digest %x", gID, d)
			continue // should never happen
		}
		row.LabelBefore = label
	}
	return nil
}

// writeDeltas writes the given rows to the SQL DB.
func writeDeltas(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "writeDeltas")
	defer span.End()

	const statement = `INSERT INTO ExpectationDeltas
(expectation_record_id, grouping_id, digest, label_before, label_after) VALUES `
	const valuesPerRow = 5
	vp := sql.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, d.ExpectationRecordID, d.GroupingID, d.Digest, d.LabelBefore, d.LabelAfter)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// writeExpectations writes expectations based on the passed in deltas to the DB.
func writeExpectations(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow) error {
	ctx, span := trace.StartSpan(ctx, "writeExpectations")
	defer span.End()

	const statement = `UPSERT INTO Expectations
(grouping_id, digest, label, expectation_record_id) VALUES `
	const valuesPerRow = 4
	vp := sql.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, d.GroupingID, d.Digest, d.LabelAfter, d.ExpectationRecordID)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// writeSecondaryExpectations writes expectations based on the passed in deltas to the appropriate
// table for the secondary branch.
func writeSecondaryExpectations(ctx context.Context, tx pgx.Tx, deltas []schema.ExpectationDeltaRow, branch string) error {
	ctx, span := trace.StartSpan(ctx, "writeSecondaryExpectations")
	defer span.End()

	const statement = `UPSERT INTO SecondaryBranchExpectations
(branch_name, grouping_id, digest, label, expectation_record_id) VALUES `
	const valuesPerRow = 5
	vp := sql.ValuesPlaceholders(valuesPerRow, len(deltas))
	arguments := make([]interface{}, 0, len(deltas)*valuesPerRow)
	for _, d := range deltas {
		arguments = append(arguments, branch, d.GroupingID, d.Digest, d.LabelAfter, d.ExpectationRecordID)
	}
	_, err := tx.Exec(ctx, statement+vp, arguments...)
	return err // don't wrap, could be retryable
}

// QueryLog returns the result from the underlying Firestore DB.
func (i *Impl) QueryLog(ctx context.Context, offset, n int, details bool) ([]expectations.TriageLogEntry, int, error) {
	return i.LegacyStore.QueryLog(ctx, offset, n, details)
}

// UndoChange undoes the change in the Firestore DB, and writes a row to a table in the SQL DB so
// it can be manually applied. This is necessary due to the incompatibility of the ID types between
// Firestore and the Expectation table. It happens rarely (~1/week across all instances), so this
// shouldn't be too large of a manual change when removing the Firestore DB.
func (i *Impl) UndoChange(ctx context.Context, changeID, userID string) error {
	err := i.LegacyStore.UndoChange(ctx, changeID, userID)
	if err != nil {
		return skerr.Wrap(err)
	}
	const statement = `INSERT INTO DeprecatedExpectationUndos (expectation_id, user_id, ts)
VALUES ($1, $2, $3)`
	_, err = i.sqlDB.Exec(ctx, statement, changeID, userID, now(ctx))
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// ForChangelist returns the result from the underlying Firestore DB wrapped so it too will
// write to the SQL db.
func (i *Impl) ForChangelist(id, crs string) expectations.Store {
	return &Impl{
		sqlDB:       i.sqlDB,
		LegacyStore: i.LegacyStore.ForChangelist(id, crs),
		branch:      fmt.Sprintf("%s_%s", crs, id),
	}
}

// GetTriageHistory returns the result from the underlying Firestore DB.
func (i *Impl) GetTriageHistory(ctx context.Context, grouping types.TestName, digest types.Digest) ([]expectations.TriageHistory, error) {
	return i.LegacyStore.GetTriageHistory(ctx, grouping, digest)
}

func convertLabel(label expectations.Label) schema.ExpectationLabel {
	switch label {
	case expectations.Positive:
		return schema.LabelPositive
	case expectations.Negative:
		return schema.LabelNegative
	case expectations.Untriaged:
		return schema.LabelUntriaged
	}
	sklog.Warningf("Invalid label %q", label)
	return schema.LabelUntriaged
}

// Make sure Impl fulfills the expectations.Store interface
var _ expectations.Store = (*Impl)(nil)

// overwriteNowKey is used by tests to make the time deterministic.
const overwriteNowKey = contextKey("overwriteNow")

type contextKey string

// now returns the current time or the time from the context.
func now(ctx context.Context) time.Time {
	if ts := ctx.Value(overwriteNowKey); ts != nil {
		return ts.(time.Time)
	}
	return time.Now()
}
