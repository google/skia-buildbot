// Command line tool to extract the source images from the fiddle 1.0 MySQL
// database and write them to their new home in Google Storage.
//
// Never actually run this program. It is here for historical reference.  The
// images have already been imported and then hand filtered, so running this
// will undo all that work.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	_ "github.com/go-sql-driver/mysql"
	"go.skia.org/infra/fiddle/go/store"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

// Command line flags.
var (
	password = flag.String("password", "", "MySQL password")
	confirm  = flag.Bool("i_really_want_to_run_this_despite_the_consequences", false, "This app shouldn't really ever need to be run since all the images have been migrated from fiddle 1.0 storage.")
)

func migrateSources(db *sql.DB, st *storage.Client) error {
	rows, err := db.Query("SELECT id, image, create_ts FROM source_images ORDER BY create_ts DESC")
	if err != nil {
		return fmt.Errorf("Query failed: %s\n", err)
	}
	defer util.Close(rows)
	bucket := st.Bucket(store.FIDDLE_STORAGE_BUCKET)
	max := 0
	for rows.Next() {
		var id int
		var image []byte
		var create_ts time.Time
		if err := rows.Scan(&id, &image, &create_ts); err != nil {
			sklog.Errorf("Failed to fetch image from database: %s", err)
			continue
		}
		sklog.Infof("Migrating %d", id)
		if id > max {
			max = id
		}
		w := bucket.Object(fmt.Sprintf("source/%d.png", id)).NewWriter(context.Background())
		w.ObjectAttrs.ContentType = "image/png"
		if _, err := w.Write(image); err != nil {
			sklog.Errorf("Failed to write %d: %s", id, err)
		}
		util.Close(w)
	}
	w := bucket.Object("source/lastid.txt").NewWriter(context.Background())
	w.ObjectAttrs.ContentType = "text/plain"
	if _, err := w.Write([]byte(fmt.Sprintf("%d", max))); err != nil {
		sklog.Errorf("Failed to write lastid.txt: %s", err)
	}
	util.Close(w)
	return nil
}

func main() {
	common.Init()
	if !*confirm {
		sklog.Fatalf("The --i_really_want_to_run_this_despite_the_consequences flag is required.")
	}
	db, err := sql.Open("mysql", fmt.Sprintf("webtry:%s@tcp(173.194.83.52:3306)/webtry?parseTime=true", *password))
	if err != nil {
		sklog.Fatalf("ERROR: Failed to open connection to SQL server: %q\n", err)
	}
	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatalf("Problem setting up client OAuth: %s", err)
	}
	storageClient, err := storage.NewClient(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		sklog.Fatalf("Problem authenticating: %s", err)
	}
	if err := migrateSources(db, storageClient); err != nil {
		sklog.Fatalf("Migration failed: %s", err)
	}
}
