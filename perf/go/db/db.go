package db

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

import (
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
)

import (
	"skia.googlesource.com/buildbot.git/perf/go/config"
	"skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	// DB is the sql database where we have commit and annotation information stored.
	DB *sql.DB = nil
)

// Init must be called once before DB is used.
//
// Since it used glog, make sure it is also called after flag.Parse is called.
func Init() {
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

		sql = `CREATE TABLE shortcuts (
            id      INTEGER     NOT NULL PRIMARY KEY AUTOINCREMENT,
            traces  MEDIUMTEXT  NOT NULL
            )`

		_, err = DB.Exec(sql)
		glog.Infoln("Status creating sqlite table for shortcuts:", err)
	}

	// Ping the database to keep the connection fresh.
	go func() {
		c := time.Tick(1 * time.Minute)
		for _ = range c {
			if err := DB.Ping(); err != nil {
				glog.Warningln("Database failed to respond:", err)
			}
			glog.Infof("db: Successful ping")
		}
	}()
}

// ReadCommitsFromDB Gets commit information from SQL database and returns a
// slice of *Commit in reverse timestamp order.
//
// TODO(bensong): read in a range of commits instead of the whole history.
func ReadCommitsFromDB() ([]*types.Commit, error) {
	glog.Infoln("readCommitsFromDB starting")
	sql := fmt.Sprintf(`SELECT
	     ts, githash, gitnumber, author, message
	     FROM githash
	     WHERE ts >= '%s'
	     ORDER BY ts DESC`, config.BEGINNING_OF_TIME.SqlTsColumn())
	s := make([]*types.Commit, 0)
	rows, err := DB.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("Failed to query githash table: %s", err)
	} else {
		glog.Infoln("executed query")
	}

	for rows.Next() {
		var ts time.Time
		var githash string
		var gitnumber int64
		var author string
		var message string
		if err := rows.Scan(&ts, &githash, &gitnumber, &author, &message); err != nil {
			glog.Errorf("Commits row scan error: ", err)
			continue
		}
		commit := types.NewCommit()
		commit.CommitTime = ts.Unix()
		commit.Hash = githash
		commit.GitNumber = gitnumber
		commit.Author = author
		commit.CommitMessage = message
		s = append(s, commit)
	}

	return s, nil
}

// AnnotationOp holds information about annotation operations.
//
// "Operation" is one of "add", "edit", "delete".
type AnnotationOp struct {
	Annotation types.Annotation `json:"annotation"`
	Operation  string           `json:"operation"`
	Hashes     []string         `json:"hashes"`
}

// validateAnnotation pre-checks if an AnnotationOp object looks valid.
//
// This does not guarantee no database operation errors, as the data may be
// missing or have foreign key restrictions, but it should catch most common
// mistakes.
//
// Returns error with message, nil if it does not detect any violations.
func validateAnnotation(op AnnotationOp) (err error) {
	switch op.Operation {
	case "add":
		// "add" must have nonempty Notes and Hashes.
		if strings.TrimSpace(op.Annotation.Notes) == "" || len(op.Hashes) == 0 {
			return fmt.Errorf("'add' operation cannot have empty Notes or Hashes.")
		}
	case "edit":
		// "edit" must have non-negative ID and nonempty Notes.
		if strings.TrimSpace(op.Annotation.Notes) == "" || op.Annotation.ID < 0 {
			return fmt.Errorf("'edit' operation cannot have empty Notes '%s' or negative ID %d.", op.Annotation.Notes, op.Annotation.ID)
		}
	case "delete":
		// "delete" must have non-negative ID.
		if op.Annotation.ID < 0 {
			return fmt.Errorf("'delete' operation cannot have negative ID: %d.", op.Annotation.ID)
		}
	default:
		return fmt.Errorf("Unknown operation: ", op.Operation)
	}
	return nil
}

// ApplyAnnotation makes changes in the annotation DB based on the given data.
func ApplyAnnotation(buf *bytes.Buffer) (err error) {
	op := AnnotationOp{}
	if err := json.Unmarshal(buf.Bytes(), &op); err != nil {
		return fmt.Errorf("Failed to unmarshal the annotation: %s", err)
	}
	if err := validateAnnotation(op); err != nil {
		return fmt.Errorf("Invalid AnnotationOp: %s", err)
	}

	switch op.Operation {
	case "add":
		// Use transaction to ensure atomic operations.
		if _, err := DB.Exec("BEGIN"); err != nil {
			return fmt.Errorf("Error starting transaction in add: ", err)
		}
		res, err := DB.Exec(`INSERT INTO notes
		     (type, author, notes)
		     VALUES (?, ?, ?)`, op.Annotation.Type, op.Annotation.Author, op.Annotation.Notes)
		if err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error executing sql: ", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error getting LastInsertId: ", err)
		}
		for i := range op.Hashes {
			_, err := DB.Exec(`INSERT INTO githashnotes
			     (githash, ts, id)
			     VALUES (?,
				     (SELECT ts FROM githash WHERE githash=?),
			             ?)`, op.Hashes[i], op.Hashes[i], id)
			if err != nil {
				DB.Exec("ROLLBACK")
				return fmt.Errorf("Error executing sql: ", err)
			}
		}
		if _, err := DB.Exec("COMMIT"); err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error commiting transaction in add: ", err)
		}
		return nil
	case "edit":
		_, err := DB.Exec(`UPDATE notes
		     SET notes  = ?,
                         author = ?,
			 type   = ?
		     WHERE id = ?`, op.Annotation.Notes, op.Annotation.Author, op.Annotation.Type, op.Annotation.ID)
		if err != nil {
			return fmt.Errorf("Error executing sql: ", err)
		}
		return nil
	case "delete":
		if _, err := DB.Exec("BEGIN"); err != nil {
			return fmt.Errorf("Error starting transaction in delete: ", err)
		}
		if _, err := DB.Exec(`DELETE FROM notes
		     WHERE id = ?`, op.Annotation.ID); err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error executing sql: ", err)
		}
		if _, err := DB.Exec(`DELETE FROM githashnotes
		     WHERE id = ?`, op.Annotation.ID); err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error executing sql: ", err)
		}
		if _, err := DB.Exec("COMMIT"); err != nil {
			DB.Exec("ROLLBACK")
			return fmt.Errorf("Error commiting transaction in delete: ", err)
		}
		return nil
	default:
		return fmt.Errorf("Unknown operation: ", op.Operation)
	}
	return nil
}

// GetAnnotations returns Annotations in JSON format for the given range of
// timestamps.
func GetAnnotations(startTS, endTS int64) ([]byte, error) {
	m := make(map[string][]*types.Annotation)
	// Gets annotations and puts them into map m.
	rows, err := DB.Query(`SELECT
	    githashnotes.githash, githashnotes.id,
	    notes.type, notes.author, notes.notes
	    FROM githashnotes
	    INNER JOIN notes
	    ON githashnotes.id=notes.id
	    WHERE githashnotes.ts >= ?
	    AND githashnotes.ts <= ?
	    ORDER BY githashnotes.id`, time.Unix(startTS, 0).Format("2006-01-02 15:04:05"), time.Unix(endTS, 0).Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("Failed to query annotations: %s", err)
	}

	for rows.Next() {
		var githash string
		var id int
		var notetype int
		var author string
		var notes string
		if err := rows.Scan(&githash, &id, &notetype, &author, &notes); err != nil {
			return nil, fmt.Errorf("Annotations row scan error: %s", err)
		}
		if _, ok := m[githash]; !ok {
			m[githash] = make([]*types.Annotation, 0)
		}
		annotation := types.Annotation{id, notes, author, notetype}
		m[githash] = append(m[githash], &annotation)
	}

	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal JSON annotations: %s", err)
	}
	return b, nil
}
