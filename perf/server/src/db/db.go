package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
)

var (
	// DB is the sql database where we have commit and annotation information stored.
	DB *sql.DB = nil
)

func init() {
	// Connect to MySQL server. First, get the password from the metadata server.
	// See https://developers.google.com/compute/docs/metadata#custom.
	req, err := http.NewRequest("GET", "http://metadata/computeMetadata/v1/instance/attributes/readwrite", nil)
	if err != nil {
		glog.Fatalln(err)
	}
	client := http.Client{}
	req.Header.Add("X-Google-Metadata-Request", "True")
	if resp, err := client.Do(req); err == nil {
		password, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			glog.Fatalln("Failed to read password from metadata server:", err)
		}
		// The IP address of the database is found here:
		//    https://console.developers.google.com/project/31977622648/sql/instances/skiaperf/overview
		// And 3306 is the default port for MySQL.
		DB, err = sql.Open("mysql", fmt.Sprintf("readwrite:%s@tcp(173.194.104.24:3306)/skia?parseTime=true", password))
		if err != nil {
			glog.Fatalln("Failed to open connection to SQL server:", err)
		}
	} else {
		glog.Infoln("Failed to find metadata, unable to connect to MySQL server (Expected when running locally):", err)
		// Fallback to sqlite for local use.
		DB, err = sql.Open("sqlite3", "./perf.db")
		if err != nil {
			glog.Fatalln("Failed to open:", err)
		}
		sql := `CREATE TABLE notes (
	     id     INT       NOT NULL PRIMARY KEY,
	     type   TINYINT,
	     author TEXT,
	     notes  TEXT      NOT NULL
	     )`
		_, err = DB.Exec(sql)
		glog.Infoln("Status creating sqlite table for notes:", err)
		sql = `CREATE TABLE githash (
	     githash   VARCHAR(40)  NOT NULL PRIMARY KEY,
	     ts        TIMESTAMP    NOT NULL,
	     gitnumber INT          NOT NULL,
	     author    TEXT         NOT NULL,
	     message   TEXT         NOT NULL
	     )`

		_, err = DB.Exec(sql)
		glog.Infoln("Status creating sqlite table for githash:", err)

		sql = `CREATE TABLE githashnotes (
	     githash VARCHAR(40)  NOT NULL,
	     ts      TIMESTAMP    NOT NULL,
	     id      INT          NOT NULL,
	     FOREIGN KEY (githash) REFERENCES githash(githash),
	     FOREIGN KEY (id) REFERENCES notes(id)
	     )`

		_, err = DB.Exec(sql)
		glog.Infoln("Status creating sqlite table for githashnotes:", err)
	}

	// Ping the database to keep the connection fresh.
	go func() {
		c := time.Tick(1 * time.Minute)
		for _ = range c {
			if err := DB.Ping(); err != nil {
				glog.Warningln("Database failed to respond:", err)
			}
		}
	}()
}
