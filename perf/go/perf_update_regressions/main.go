// Update the regression table so that Regressions are indexed
// by alerts.Config.ID and not by Query.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/db"
	idb "go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/regression"
)

var (
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	promptPassword = flag.Bool("password", false, "Prompt for root password.")
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

func main() {
	defer common.LogPanic()
	// Set up flags.
	dbConf := idb.DBConfigFromFlags()

	// Global init to initialize glog and parse arguments.
	common.Init()

	if *promptPassword {
		if err := dbConf.PromptForPassword(); err != nil {
			sklog.Fatal(err)
		}
	}
	if err := dbConf.InitDB(); err != nil {
		sklog.Fatal(err)
	}
	alertStore := alerts.NewStore()
	cfg, err := alertStore.List(false)
	if err != nil {
		sklog.Fatalf("Failed to retrieve alerts: %s", err)
	}
	if len(cfg) == 0 {
		sklog.Fatal("No alerts.Config's found.")
	}
	// Create map from query to key.
	keyFrom := map[string]string{}
	for _, c := range cfg {
		keyFrom[c.Query] = c.IdAsString()
	}

	err = intx(func(tx *sql.Tx) error {

		// Migrate the regressions db.
		rows, err := tx.Query("SELECT cid, body FROM regression")
		if err != nil {
			return fmt.Errorf("Failed to query database.")
		}

		//          map[cid]body
		readRows := map[string]*regression.Regressions{}
		for rows.Next() {
			var id string
			var body string
			if err := rows.Scan(&id, &body); err != nil {
				return fmt.Errorf("Failed to read from database: %s", err)
			}
			reg := regression.New()

			if err := json.Unmarshal([]byte(body), reg); err != nil {
				return fmt.Errorf("Failed to decode JSON body: %s", err)
			}
			readRows[id] = reg
		}
		util.Close(rows)
		for id, reg := range readRows {
			newByAlertID := map[string]*regression.Regression{}
			for query, r := range reg.ByAlertID {
				sklog.Infof("cid=%s - query=%q", id, query)
				newKey, ok := keyFrom[query]
				if !ok {
					sklog.Warning("Could not find matching key for query: %q", query)
					continue
				}
				newByAlertID[newKey] = r
			}
			reg.ByAlertID = newByAlertID

			newBody, err := reg.JSON()
			if err != nil {
				return fmt.Errorf("Failed to encode body: %s", err)
			}
			// MEDIUMTEXT is only 16MB, and will silently be truncated.
			if len(newBody) > 16777215 {
				return fmt.Errorf("Regressions is too large, >16 MB.")
			}
			if err != nil {
				return fmt.Errorf("Failed to encode Regressions to JSON: %s", err)
			}
			_, err = tx.Exec("UPDATE regression SET body=? WHERE cid=?", newBody, id)
			if err != nil {
				return fmt.Errorf("Failed to write to database: %s", err)
			}
		}
		return nil
	})
	if err != nil {
		sklog.Fatalf("Failed migration: %s", err)
	}
}
