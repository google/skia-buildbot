// Package sqlalertstore implements alerts.Store using SQL.
//
// Please see perf/sql/migrations for the database schema used.
package sqlalertstore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	perfsql "go.skia.org/infra/perf/go/sql"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertAlert statement = iota
	updateAlert
	deleteAlert
	listActiveAlerts
	listAllAlerts
)

// statements holds all the raw SQL statements used.
var statements = map[statement]string{
	insertAlert: `
		INSERT INTO
			Alerts (alert, last_modified)
		VALUES
			($1, $2)
		`,
	updateAlert: `
		UPSERT INTO
			Alerts (id, alert, config_state, last_modified)
		VALUES
			($1, $2, $3, $4)
		`,
	deleteAlert: `
		UPDATE
		  	Alerts
		SET
			config_state=1, -- alerts.DELETED
			last_modified=$1
		WHERE
			id=$2
		`,
	listActiveAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		WHERE
			config_state=0 -- alerts.ACTIVE
		`,
	listAllAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		`,
}

// SQLAlertStore implements the alerts.Store interface.
type SQLAlertStore struct {
	// db is the database interface.
	db *pgxpool.Pool
}

// New returns a new *SQLAlertStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *pgxpool.Pool, dialect perfsql.Dialect) (*SQLAlertStore, error) {

	return &SQLAlertStore{
		db: db,
	}, nil
}

// Save implements the alerts.Store interface.
func (s *SQLAlertStore) Save(ctx context.Context, cfg *alerts.Alert) error {
	cfg.SetIDFromString(cfg.IDAsString)
	b, err := json.Marshal(cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize Alert for saving with ID=%d", cfg.ID)
	}
	now := time.Now().Unix()
	if cfg.ID == alerts.BadAlertID {
		// Not a valid ID, so this should be an insert, not an update.
		if _, err := s.db.Exec(ctx, statements[insertAlert], string(b), now); err != nil {
			return skerr.Wrapf(err, "Failed to insert alert")
		}
	} else {
		if _, err := s.db.Exec(ctx, statements[updateAlert], cfg.ID, string(b), cfg.StateToInt(), now); err != nil {
			return skerr.Wrapf(err, "Failed to update Alert with ID=%d", cfg.ID)
		}
	}
	return nil
}

// Delete implements the alerts.Store interface.
func (s *SQLAlertStore) Delete(ctx context.Context, id int) error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(ctx, statements[deleteAlert], now, id); err != nil {
		return skerr.Wrapf(err, "Failed to mark Alert as deleted with ID=%d", id)
	}
	return nil
}

// List implements the alerts.Store interface.
func (s *SQLAlertStore) List(ctx context.Context, includeDeleted bool) ([]*alerts.Alert, error) {
	stmt := listActiveAlerts
	if includeDeleted {
		stmt = listAllAlerts
	}
	rows, err := s.db.Query(ctx, statements[stmt])
	if err != nil {
		return nil, err
	}
	ret := []*alerts.Alert{}
	for rows.Next() {
		var id int64
		var serializedAlert string
		if err := rows.Scan(&id, &serializedAlert); err != nil {
			return nil, err
		}
		a := &alerts.Alert{}
		if err := json.Unmarshal([]byte(serializedAlert), a); err != nil {
			return nil, skerr.Wrapf(err, "Failed to deserialize JSON Alert.")
		}
		a.ID = id
		a.IDAsString = fmt.Sprintf("%d", id)
		ret = append(ret, a)
	}
	return ret, nil
}
