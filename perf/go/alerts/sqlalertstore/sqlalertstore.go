// Package sqlalertstore implements alerts.Store using SQL.
//
// Please see perf/sql/migrations for the database schema used.
package sqlalertstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/alerts"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertAlert statement = iota
	updateAlert
	deleteAlert
	deleteAllAlerts
	listActiveAlerts
	listAllAlerts
)

// statements holds all the raw SQL statements used.
var statements = map[statement]string{
	insertAlert: `
		INSERT INTO
			Alerts (alert, last_modified, sub_name, sub_revision)
		VALUES
			($1, $2, $3, $4)
		RETURNING
			id
		`,
	updateAlert: `
		UPSERT INTO
			Alerts (id, alert, config_state, last_modified, sub_name, sub_revision)
		VALUES
			($1, $2, $3, $4, $5, $6)
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
	deleteAllAlerts: `
		UPDATE
			Alerts
		SET
			config_state=1, -- alerts.DELETED
			last_modified=$1
		WHERE
			config_state=0 -- alerts.ACTIVE
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
	db pool.Pool
}

// New returns a new *SQLAlertStore.
//
// We presume all migrations have been run against db before this function is
// called.
func New(db pool.Pool) (*SQLAlertStore, error) {
	return &SQLAlertStore{
		db: db,
	}, nil
}

// Save implements the alerts.Store interface.
func (s *SQLAlertStore) Save(ctx context.Context, req *alerts.SaveRequest) error {

	cfg := req.Cfg
	b, err := json.Marshal(cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize Alert for saving with ID=%s", cfg.IDAsString)
	}
	now := time.Now().Unix()

	if cfg.IDAsString == alerts.BadAlertIDAsAsString {
		newID := alerts.BadAlertID
		// Not a valid ID, so this should be an insert, not an update.
		if err := s.db.QueryRow(ctx, statements[insertAlert], string(b), now, nil, nil).Scan(&newID); err != nil {
			return skerr.Wrapf(err, "Failed to insert alert")
		}
		cfg.SetIDFromInt64(newID)
	} else {
		nameOrNull := sql.NullString{Valid: false}
		revisionOrNull := sql.NullString{Valid: false}

		if req.SubKey != nil {
			nameOrNull.String = req.SubKey.SubName
			nameOrNull.Valid = true
			revisionOrNull.String = req.SubKey.SubRevision
			revisionOrNull.Valid = true
		}
		if _, err := s.db.Exec(ctx, statements[updateAlert], cfg.IDAsStringToInt(), string(b), cfg.StateToInt(), now, nameOrNull, revisionOrNull); err != nil {
			return skerr.Wrapf(err, "Failed to update Alert with ID=%s", cfg.IDAsString)
		}
	}

	return nil
}

// ReplaceAll implements the alerts.Store interface.
// TODO(eduardoyap): Modify to execute one Insert statement, instead of multiple.
func (s *SQLAlertStore) ReplaceAll(ctx context.Context, reqs []*alerts.SaveRequest) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	now := time.Now().Unix()
	if _, err := tx.Exec(ctx, statements[deleteAllAlerts], now); err != nil {
		if err := tx.Rollback(ctx); err != nil {
			sklog.Errorf("Failed on rollback: %s", err)
		}
		return skerr.Wrap(err)
	}

	for _, req := range reqs {
		cfg := req.Cfg
		b, err := json.Marshal(cfg)
		if err != nil {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return skerr.Wrap(err)
		}

		if _, err := tx.Exec(ctx, statements[insertAlert], string(b), now, req.SubKey.SubName, req.SubKey.SubRevision); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return skerr.Wrap(err)
		}
	}
	return tx.Commit(ctx)
}

// Delete implements the alerts.Store interface.
func (s *SQLAlertStore) Delete(ctx context.Context, id int) error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(ctx, statements[deleteAlert], now, id); err != nil {
		return skerr.Wrapf(err, "Failed to mark Alert as deleted with ID=%d", id)
	}
	return nil
}

type sortableAlertSlice []*alerts.Alert

func (p sortableAlertSlice) Len() int { return len(p) }
func (p sortableAlertSlice) Less(i, j int) bool {
	if p[i].DisplayName == p[j].DisplayName {
		return p[i].IDAsString < p[j].IDAsString
	}
	return p[i].DisplayName < p[j].DisplayName
}
func (p sortableAlertSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

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
		a.SetIDFromInt64(id)
		ret = append(ret, a)
	}
	sort.Sort(sortableAlertSlice(ret))
	return ret, nil
}
