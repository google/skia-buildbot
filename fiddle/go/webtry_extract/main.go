// Migration utility that extracts fiddles from MySQL and writes
// then to the new fiddle 2.0 store. This does not run the fiddles,
// that's handled by a separate application that can be used
// to find broken fiddles.
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/fiddle/go/types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

// Command line flags.
var (
	password = flag.String("password", "", "MySQL password")
	gitHash  = flag.String("git_hash", "", "Git hash to use for the upload.")
)

type Fiddle struct {
	Code   string
	Width  int
	Height int
	Source int
}

// migrateCode copies all existing fiddles from the MySQL database to Google Storage.
func migrateCode(db *sql.DB, st *store.Store, ts time.Time, hash string) error {
	fiddles := []*Fiddle{}
	code := ""
	count := 0
	width := 0
	height := 0
	source := 0

	if err := db.QueryRow("SELECT COUNT(*) FROM webtry").Scan(&count); err != nil {
		return fmt.Errorf("Failed to retrieve try count: %s", err)
	}
	glog.Infof("Total fiddles: %d", count)

	// Migrating takes too long the MySQL connection fails, so just read all the data
	// into memory and then write it out to Google Storage.
	rows, err := db.Query("SELECT code, width, height, source_image_id FROM webtry")
	if err != nil {
		return fmt.Errorf("Failed to retrieve try: %s", err)
	}
	for rows.Next() {
		if err := rows.Scan(&code, &width, &height, &source); err != nil {
			return fmt.Errorf("Failed to scan row for code: %s", err)
		}
		fiddles = append(fiddles, &Fiddle{
			Code:   code,
			Width:  width,
			Height: height,
			Source: source,
		})
	}
	util.Close(rows)
	glog.Infof("Loaded all fiddles.")
	for i, f := range fiddles {
		glog.Infof("Migrating %d", i)
		options := types.Options{
			Width:  f.Width,
			Height: f.Height,
			Source: f.Source,
		}
		codeHash, err := st.Put(f.Code, options, hash, ts, nil)
		if err != nil {
			glog.Infof("Failed to write code for %s: %s\n: %s", codeHash, code, err)
		}
	}
	return nil
}

func main() {
	common.Init()
	if *gitHash == "" {
		glog.Fatalf("The --git_hash flag is required.")
	}
	db, err := sql.Open("mysql", fmt.Sprintf("webtry:%s@tcp(173.194.83.52:3306)/webtry?parseTime=true", *password))
	if err != nil {
		glog.Fatalf("ERROR: Failed to open connection to SQL server: %q\n", err)
	}
	st, err := store.New()
	if err != nil {
		glog.Fatalf("Failed to create connnetion to Google Storage: %s", err)
	}
	ts := time.Now()
	if err := migrateCode(db, st, ts, *gitHash); err != nil {
		glog.Fatalf("Migration failed: %s", err)
	}
}
