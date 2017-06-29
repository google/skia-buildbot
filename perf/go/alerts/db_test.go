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
