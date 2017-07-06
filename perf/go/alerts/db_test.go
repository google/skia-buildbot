package alerts

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/db"
	sqlmock "gopkg.in/DATA-DOG/go-sqlmock.v1"
)

func TestStoreNew(t *testing.T) {
	testutils.SmallTest(t)

	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	a := NewStore()
	cfg := NewConfig()
	cfg.Query = "source_type=svg"
	body, err := json.Marshal(cfg)
	assert.NoError(t, err)

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO alerts").WithArgs(cfg.State, body).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Put mock db into place.
	db.DB = mdb

	err = a.Save(cfg)
	assert.NoError(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func TestStoreUpdate(t *testing.T) {
	testutils.SmallTest(t)

	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	a := NewStore()
	cfg := NewConfig()
	cfg.ID = 1
	cfg.Query = "source_type=svg"
	body, err := json.Marshal(cfg)
	assert.NoError(t, err)

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts SET state=(.+), body=(.+) WHERE id=(.+)").WithArgs(cfg.State, body, cfg.ID).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Put mock db into place.
	db.DB = mdb

	err = a.Save(cfg)
	assert.NoError(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func TestDelete(t *testing.T) {
	testutils.SmallTest(t)

	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	a := NewStore()

	// Set expectations.
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE alerts set state=(.+) WHERE id=(.+)").WithArgs(DELETED, 2).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	// Put mock db into place.
	db.DB = mdb

	err = a.Delete(2)
	assert.NoError(t, err)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func TestListAll(t *testing.T) {
	testutils.SmallTest(t)

	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	a := NewStore()
	cfg := NewConfig()
	cfg.Query = "source_type=svg"
	body1, err := json.Marshal(cfg)
	assert.NoError(t, err)
	cfg.Query = "source_type=skp"
	body2, err := json.Marshal(cfg)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "state", "body"}).
		AddRow("2", ACTIVE, body1).
		AddRow("1", DELETED, body2)

	// Set expectations.
	mock.ExpectQuery("SELECT id, state, body FROM alerts ORDER BY id ASC").WillReturnRows(rows)

	// Put mock db into place.
	db.DB = mdb

	list, err := a.List(true)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(list))
	assert.Equal(t, DELETED, list[1].State)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}

func TestListOnlyActive(t *testing.T) {
	testutils.SmallTest(t)

	mdb, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	a := NewStore()
	cfg := NewConfig()
	cfg.Query = "source_type=svg"
	body1, err := json.Marshal(cfg)
	assert.NoError(t, err)
	cfg.Query = "source_type=skp"
	body2, err := json.Marshal(cfg)
	assert.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "state", "body"}).
		AddRow("2", ACTIVE, body1).
		AddRow("1", ACTIVE, body2)

	// Set expectations.
	mock.ExpectQuery("SELECT id, state, body FROM alerts WHERE state=(.+) ORDER BY id ASC").WithArgs(ACTIVE).WillReturnRows(rows)

	// Put mock db into place.
	db.DB = mdb

	list, err := a.List(false)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(list))
	assert.Equal(t, 2, list[0].ID)
	assert.Equal(t, "source_type=svg", list[0].Query)
	assert.Equal(t, 1, list[1].ID)
	assert.Equal(t, "source_type=skp", list[1].Query)

	// Make sure that all expectations were met.
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expections: %s", err)
	}
}
