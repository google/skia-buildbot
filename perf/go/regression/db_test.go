package regression

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/db"

	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

// TestSetLowWithMissing test storing a low cluster to the sql database.
func TestSetLowWithMissing(t *testing.T) {
	testutils.SmallTest(t)
	// Set up mock db.
	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mdb.Close()

	r := New()
	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Source: "master",
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("source_type=skp", df, cl)

	// body is what our expected body should look like after adding the low cluster.
	body, err := r.JSON()
	assert.NoError(t, err)

	// We don't add any row data, so the query returns empty, forcing SetLow to
	// create a new Regressions.
	rows := sqlmock.NewRows([]string{"cid", "body"})

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT cid, body FROM regression").WillReturnRows(rows)
	mock.ExpectExec("INSERT INTO regression").WithArgs(c.ID(), c.Timestamp, r.Triaged(), body, c.ID(), c.Timestamp, r.Triaged(), body).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Put mock db into place.
	db.DB = mdb

	// Execute our method.
	st := NewStore()
	err = st.SetLow(c, "source_type=skp", df, cl)
	assert.NoError(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

// TestTriageWithExisting test successfully triaging a low cluster in the sql database.
func TestTriageWithExisting(t *testing.T) {
	testutils.SmallTest(t)
	// Set up mock db.
	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mdb.Close()

	r := New()
	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Source: "master",
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r.SetLow("source_type=skp", df, cl)

	body, err := r.JSON()
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"cid", "body"}).AddRow(c.ID(), body)

	// Now determine what the serialized Regressions would look like
	// after a successful triaging.
	tr := TriageStatus{
		Status:  POSITIVE,
		Message: "SKP Update",
	}
	err = r.TriageLow("source_type=skp", tr)
	assert.NoError(t, err)
	body, err = r.JSON()
	assert.NoError(t, err)

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT cid, body FROM regression").WillReturnRows(rows)
	mock.ExpectExec("INSERT INTO regression").WithArgs(c.ID(), c.Timestamp, r.Triaged(), body, c.ID(), c.Timestamp, r.Triaged(), body).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Put mock db into place.
	db.DB = mdb

	// Execute our method.
	st := NewStore()
	err = st.TriageLow(c, "source_type=skp", tr)
	assert.NoError(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

// TestTriageWithMissing tries to triage a cluster that doesn't exist, which
// should fail and result in a rollback.
func TestTriageWithMissing(t *testing.T) {
	testutils.SmallTest(t)
	// Set up mock db.
	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mdb.Close()

	c := &cid.CommitDetail{
		CommitID: cid.CommitID{
			Source: "master",
			Offset: 1,
		},
		Timestamp: 1479235651,
	}

	// Note no rows are added, so trying to triage a missing cluster should fail.
	rows := sqlmock.NewRows([]string{"cid", "body"})

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT cid, body FROM regression").WillReturnRows(rows)
	mock.ExpectRollback()

	// Put mock db into place.
	db.DB = mdb

	// Execute our method.
	st := NewStore()
	tr := TriageStatus{
		Status:  POSITIVE,
		Message: "SKP Update",
	}
	err = st.TriageLow(c, "source_type=skp", tr)
	assert.Error(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func TestRange(t *testing.T) {
	testutils.SmallTest(t)
	// Set up mock db.
	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer mdb.Close()

	r1 := New()
	c1 := cid.CommitID{
		Source: "master",
		Offset: 1,
	}
	r2 := New()
	c2 := cid.CommitID{
		Source: "master",
		Offset: 2,
	}
	df := &dataframe.FrameResponse{}
	cl := &clustering2.ClusterSummary{}
	r2.SetLow("source_type=skp", df, cl)

	body1, err := r1.JSON()
	assert.NoError(t, err)
	body2, err := r2.JSON()
	assert.NoError(t, err)

	// Set expectations.
	rows := sqlmock.NewRows([]string{"cid", "timestamp", "body"}).
		AddRow("master-000001", 1479235651, body1).
		AddRow("master-000002", 1479235789, body2)

	mock.ExpectQuery("SELECT cid, timestamp, body FROM regression WHERE timestamp >= (.+) AND timestamp < (.+) ORDER BY timestamp").WillReturnRows(rows)

	// Put mock db into place.
	db.DB = mdb

	// Execute our method.
	st := NewStore()
	reg, err := st.Range(1479235651, 1479235999)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(reg))
	assert.True(t, reg[c1.ID()].Triaged())
	assert.False(t, reg[c2.ID()].Triaged())
	assert.Equal(t, 1, len(reg[c2.ID()].ByQuery))

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
