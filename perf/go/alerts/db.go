// Save and retrieve alerts.Config's to/from a database.
//
// TODO(jcgregorio) Add a cleanup process that removes DELETED configs from the
// database after a long period of time, using the lastmodified timestamp.
package alerts

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/db"
)

// intx runs f within a database transaction.
//
func intx(f func(tx *sql.Tx) error) (err error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("Failed to start transaction: %s", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	err = f(tx)
	return err
}

// Store persists Config's to/from an SQL database.
type Store struct {
}

// NewStore returns a new Store.
func NewStore() *Store {
	return &Store{}
}

// Save can write a new, or update an existing, Config. New
// Config's will have an ID of -1.
func (s *Store) Save(cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("Failed to save invalid Config: %s", err)
	}
	return intx(func(tx *sql.Tx) error {
		body, err := json.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("Failed to encode alerts.Config to JSON: %s", err)
		}
		// MEDIUMTEXT is only 16MB, and will silently be truncated.
		if len(body) > 16*1024*1024 {
			return fmt.Errorf("Regressions is too large, >16 MB.")
		}
		if cfg.ID == INVALID_ID {
			_, err = tx.Exec("INSERT INTO alerts (state, body) VALUES (?, ?)", cfg.State, body)
		} else {
			_, err = tx.Exec("UPDATE alerts SET state=?, body=? WHERE id=?", cfg.State, body, cfg.ID)
		}

		if err != nil {
			return fmt.Errorf("Failed to write to database: %s", err)
		}
		return nil
	})
}

func (s *Store) Delete(id int) error {
	return intx(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE alerts set state=? WHERE id=?", DELETED, id)
		if err != nil {
			return fmt.Errorf("Failed to write to database: %s", err)
		}
		return nil
	})
}

func (s *Store) List(includeDeleted bool) ([]*Config, error) {
	ret := []*Config{}
	var rows *sql.Rows
	var err error
	if includeDeleted {
		rows, err = db.DB.Query("SELECT id, state, body FROM alerts ORDER BY id ASC")
	} else {
		rows, err = db.DB.Query("SELECT id, state, body FROM alerts WHERE state=? ORDER BY id ASC", ACTIVE)
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	defer util.Close(rows)
	sklog.Infoln("Processing Config rows.")
	for rows.Next() {
		var id int
		var state int
		body := ""
		if err := rows.Scan(&id, &state, &body); err != nil {
			return nil, fmt.Errorf("Failed to read row from database: %s", err)
		}
		cfg := NewConfig()
		if err := json.Unmarshal([]byte(body), cfg); err != nil {
			return nil, fmt.Errorf("Failed to unmarshall row from database: %q %s", body, err)
		}
		cfg.ID = id
		cfg.State = ConfigState(state)
		if err := cfg.Validate(); err != nil {
			sklog.Errorf("Found an invalid alert %v: %s", *cfg, err)
		}
		ret = append(ret, cfg)
	}

	return ret, nil
}
