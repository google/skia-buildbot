// Package sqlalertstore implements alerts.Store using SQL.
package sqlalertstore

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"time"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
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
	listForSub
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
		INSERT INTO
			Alerts (id, alert, config_state, last_modified, sub_name, sub_revision)
		VALUES
			($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE
		SET id=EXCLUDED.id, alert=EXCLUDED.alert, config_state=EXCLUDED.config_state,
			last_modified=EXCLUDED.last_modified, sub_name=EXCLUDED.sub_name,
			sub_revision=EXCLUDED.sub_revision
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
	listForSub: `
			SELECT
				id, alert
			FROM
				ALERTS
			WHERE
				config_state=0
				AND sub_name=$1
			`,
}

// SQLAlertStore implements the alerts.Store interface.
type SQLAlertStore struct {
	// db is the database interface.
	db pool.Pool
	// statements is the SQL statements to use.
	statements map[statement]string
}

// New returns a new *SQLAlertStore.
func New(db pool.Pool) (*SQLAlertStore, error) {
	return &SQLAlertStore{
		db:         db,
		statements: statements,
	}, nil
}

// Save implements the alerts.Store interface.
func (s *SQLAlertStore) Save(ctx context.Context, req *alerts.SaveRequest) error {
	cfg := req.Cfg
	var b bytes.Buffer
	err := json.NewEncoder(&b).Encode(cfg)
	if err != nil {
		return skerr.Wrapf(err, "Failed to serialize Alert for saving with ID=%s", cfg.IDAsString)
	}
	now := time.Now().Unix()

	if cfg.IDAsString == alerts.BadAlertIDAsAsString {
		newID := alerts.BadAlertID
		// Not a valid ID, so this should be an insert, not an update.
		if err := s.db.QueryRow(ctx, s.statements[insertAlert], b.String(), now, nil, nil).Scan(&newID); err != nil {
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
		if _, err := s.db.Exec(ctx, s.statements[updateAlert], cfg.IDAsStringToInt(), b.String(), cfg.StateToInt(), now, nameOrNull, revisionOrNull); err != nil {
			return skerr.Wrapf(err, "Failed to update Alert with ID=%s", cfg.IDAsString)
		}
	}

	return nil
}

// ReplaceAll implements the alerts.Store interface.
// TODO(eduardoyap): Modify to execute one Insert statement, instead of multiple.
func (s *SQLAlertStore) ReplaceAll(ctx context.Context, reqs []*alerts.SaveRequest, tx pgx.Tx) error {

	now := time.Now().Unix()
	if _, err := tx.Exec(ctx, s.statements[deleteAllAlerts], now); err != nil {
		return skerr.Wrap(err)
	}

	var b bytes.Buffer
	jsonEncoder := json.NewEncoder(&b)
	for _, req := range reqs {
		b.Reset()
		cfg := req.Cfg
		err := jsonEncoder.Encode(cfg)
		if err != nil {
			return skerr.Wrap(err)
		}

		if _, err := tx.Exec(ctx, s.statements[insertAlert], b.String(), now, req.SubKey.SubName, req.SubKey.SubRevision); err != nil {
			return skerr.Wrap(err)
		}
	}

	return nil
}

// Delete implements the alerts.Store interface.
func (s *SQLAlertStore) Delete(ctx context.Context, id int) error {
	now := time.Now().Unix()
	if _, err := s.db.Exec(ctx, s.statements[deleteAlert], now, id); err != nil {
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
	rows, err := s.db.Query(ctx, s.statements[stmt])
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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

// ListForSubscription implements the alerts.Store interface.
func (s *SQLAlertStore) ListForSubscription(ctx context.Context, subName string) ([]*alerts.Alert, error) {
	rows, err := s.db.Query(ctx, s.statements[listForSub], subName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
