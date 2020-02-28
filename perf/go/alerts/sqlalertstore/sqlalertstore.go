// Package sqlalertstore implements alerts.Store using SQL.
package sqlalertstore

import (
	"context"
	"database/sql"
	"encoding/json"

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

// statements allows looking up raw SQL statements by their statement id.
type statements map[statement]string

// statementsByDialect holds all the raw SQL statemens used per Dialect of SQL.
var statementsByDialect = map[perfsql.Dialect]statements{
	perfsql.SQLiteDialect: {
		insertAlert: `
		INSERT INTO
			Alerts (alert)
		VALUES
			(?)`,
		updateAlert: `
		UPDATE
			  Alerts
		SET
			alert=?,
			config_state=?
		WHERE
			id=?
		`,
		deleteAlert: `
		UPDATE
		  	Alerts
		SET
			config_state=1
		WHERE
			id=?
		`,
		listActiveAlerts: `
		SELECT
			id, alert
		FROM
			Alerts
		WHERE
			config_state = 0
		`,
		listAllAlerts: `
		SELECT
			id, alert
		FROM
			Alerts
		`,
	},
	perfsql.CockroachDBDialect: {
		insertAlert: `
		INSERT INTO
			Alerts (alert)
		VALUES
			($1)
		`,
		updateAlert: `
		UPDATE
			  Alerts
		SET
			alert=$1,
			config_state=$2,
		WHERE
			id=$3
		`,
		deleteAlert: `
		UPDATE
		  	Alerts (config_state)
		SET
			config_state=$1
		WHERE
			id=$2
		`,
		listActiveAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		WHERE
			config_state = 0
		`,
		listAllAlerts: `
		SELECT
			id, alert
		FROM
			ALERTS
		`,
	},
}

// SQLAlertStore implements the alerts.Store interface.
type SQLAlertStore struct {
	// db is the database connection.
	db *sql.DB

	// preparedStatements are all the prepared SQL statements.
	preparedStatements map[statement]*sql.Stmt
}

// New returns a new *SQLAlertStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db *sql.DB, dialect perfsql.Dialect) (*SQLAlertStore, error) {
	preparedStatements := map[statement]*sql.Stmt{}
	for key, statement := range statementsByDialect[dialect] {
		prepared, err := db.Prepare(statement)
		if err != nil {
			return nil, skerr.Wrapf(err, "Failed to prepare statment %v %q", key, statement)
		}
		preparedStatements[key] = prepared
	}

	return &SQLAlertStore{
		db:                 db,
		preparedStatements: preparedStatements,
	}, nil
}

// Save implements the alerts.Store interface.
func (s *SQLAlertStore) Save(ctx context.Context, cfg *alerts.Alert) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize Alert for saving with ID=%d", cfg.ID)
	}
	if cfg.ID == alerts.INVALID_ID {
		// Not a valid ID, so this should be an insert, not an update.
		if _, err := s.preparedStatements[insertAlert].ExecContext(ctx, string(b)); err != nil {
			return skerr.Wrapf(err, "Failed to insert alert")
		}
	} else {
		if _, err := s.preparedStatements[updateAlert].ExecContext(ctx, string(b), cfg.State, cfg.ID); err != nil {
			return skerr.Wrapf(err, "Failed to update Alert with ID=%d", cfg.ID)
		}
	}
	return nil
}

// Delete implements the alerts.Store interface.
func (s *SQLAlertStore) Delete(ctx context.Context, id int) error {
	if _, err := s.preparedStatements[deleteAlert].ExecContext(ctx, id); err != nil {
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
	rows, err := s.preparedStatements[stmt].QueryContext(ctx)
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
		var a alerts.Alert
		if err := json.Unmarshal([]byte(serializedAlert), &a); err != nil {
			return nil, skerr.Wrapf(err, "Failed to deserialize JSON Alert.")
		}
		a.ID = id
		ret = append(ret, &a)
	}
	return ret, nil
}
