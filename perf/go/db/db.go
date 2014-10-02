package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
	"skia.googlesource.com/buildbot.git/perf/go/metadata"
	"skia.googlesource.com/buildbot.git/perf/go/util"
)

var (
	// DB is the sql database where we have commit and annotation information stored.
	DB *sql.DB = nil

	// Keeps track if we are connected to MySQL or SQLite
	isMySQL = true
)

const (
	// Key of the password for the readwrite user.
	METADATA_KEY = "readwrite"

	// Path where the SQLite database is stored when running locally.
	SQLITE_DB_PATH = "./perf.db"

	// Template to generate the database connection string in production.
	// The IP address of the database is found here:
	//    https://console.developers.google.com/project/31977622648/sql/instances/skiaperf/overview
	// And 3306 is the default port for MySQL.
	DB_CONN_TMPL = "readwrite:%s@tcp(173.194.104.24:3306)/skia?parseTime=true"
)

// Init must be called once before DB is used.
//
// Since it used glog, make sure it is also called after flag.Parse is called.
func Init(mysqlConnStr string) {
	// If there is a connection string then connect to the MySQL server.
	// This is for testing only. In production we get the relevant information
	// from the metadata server.
	var err error

	if mysqlConnStr != "" {
		glog.Infoln("Opening SQL database.")
		if DB, err = sql.Open("mysql", mysqlConnStr); err == nil {
			glog.Infoln("Sending Ping.")
			err = DB.Ping()
		}

		if err != nil {
			glog.Fatalln("Failed to open connection to SQL server:", err)
		}

		isMySQL = true
	} else {
		// Open a local SQLite database instead.
		glog.Infof("Opening local sqlite database at: %s", SQLITE_DB_PATH)
		// Fallback to sqlite for local use.
		DB, err = sql.Open("sqlite3", SQLITE_DB_PATH)
		if err != nil {
			glog.Fatalln("Failed to open:", err)
		}
		isMySQL = false
	}

	// Make sure the migration table exists.
	if err := checkVersionTable(); err != nil {
		// We are using panic() instead of Fataln() to be able to trap this
		// in tests and make sure it fails when no version table exists.
		glog.Errorln("Unable to create version table.")
		panic("Attempt to create version table returned: " + err.Error())
	}
	glog.Infoln("Version table OK.")

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

// Returns the DB connection string for running in production where a
// metadata server is available. If 'local' is true it will always return
// "" (empty string). When used with Init() this will cause it to use a
// local SQLite database. If it's not local and the meta data server is
// unreachable it will terminate.
func ProdConnectionString(local bool) string {
	if local {
		return ""
	}

	//  First, get the password from the metadata server.
	// See https://developers.google.com/compute/docs/metadata#custom.
	password, err := metadata.Get(METADATA_KEY)
	if err != nil {
		glog.Fatalf("Failed to find metadata. Use 'local' flag when running locally.")
	}
	return fmt.Sprintf(DB_CONN_TMPL, password)
}

// Migrates the database to the specified target version. Use DBVersion() to
// retrieve the current version of the database.
func Migrate(targetVersion int) error {
	if (targetVersion < 0) || (targetVersion > MaxDBVersion()) {
		glog.Fatalf("Target db version must be in range: [0 .. %d]", MaxDBVersion())
	}

	currentVersion, err := DBVersion()
	if err != nil {
		return err
	}

	if targetVersion == currentVersion {
		return nil
	}

	// start a transaction
	txn, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			glog.Errorf("Rolling back comit. Error: %s", err)
			txn.Rollback()
		} else {
			glog.Infoln("Committing changes.")
			txn.Commit()
		}
	}()

	// run through the transactions
	runSteps := getMigrations(isMySQL, currentVersion, targetVersion)
	if len(runSteps) == 0 {
		glog.Fatalln("Unable to find migration steps.")
	}

	for _, step := range runSteps {
		for _, stmt := range step {
			glog.Infoln("EXECUTING: \n", stmt)
			if _, err = txn.Exec(stmt); err != nil {
				return err
			}
		}
	}

	// update the dbversion table
	if err = setDBVersion(txn, targetVersion); err != nil {
		return err
	}

	return nil
}

// Returns the current version of the database. It assumes that the
// Migrate function has already been called and the version table has been
// created in the database.
func DBVersion() (int, error) {
	stmt := `
		SELECT version
		FROM sk_db_version
		WHERE id=1`

	var version int
	err := DB.QueryRow(stmt).Scan(&version)
	return version, err
}

// Returns the highest version currently available.
func MaxDBVersion() int {
	return len(migrationSteps)
}

// Returns an error if the version table does not exist.
func checkVersionTable() error {
	// Check if the table exists in MySQL or SQLite.
	stmt := "SHOW TABLES LIKE 'sk_db_version'"
	if !isMySQL {
		stmt = "SELECT name FROM sqlite_master WHERE type='table' AND name='sk_db_version';"
	}

	var temp string
	err := DB.QueryRow(stmt).Scan(&temp)
	if err != nil {
		// See if we can create the version table.
		return ensureVersionTable()
	}

	return nil
}

func setDBVersion(txn *sql.Tx, newDBVersion int) error {
	stmt := `REPLACE INTO sk_db_version (id, version, updated) VALUES(1, ?, ?)`
	_, err := txn.Exec(stmt, newDBVersion, time.Now().Unix())
	return err
}

func ensureVersionTable() error {
	txn, err := DB.Begin()
	defer func() {
		if err != nil {
			glog.Errorf("Encountered error rolling back: %s", err.Error())
			txn.Rollback()
		} else {
			txn.Commit()
		}
	}()

	if err != nil {
		fmt.Errorf("Unable to start database transaction. %s", err.Error())
	}

	stmt := `CREATE TABLE IF NOT EXISTS sk_db_version (
			id         INTEGER      NOT NULL PRIMARY KEY,
			version    INTEGER      NOT NULL,
			updated    BIGINT       NOT NULL
		)`
	if _, err = txn.Exec(stmt); err != nil {
		return fmt.Errorf("Creating version table failed: %s", err.Error())
	}

	stmt = "SELECT COUNT(*) FROM sk_db_version"
	var count int
	if err = txn.QueryRow(stmt).Scan(&count); err != nil {
		return fmt.Errorf("Unable to read version table: %s", err.Error())
	}

	// In both cases we want the transaction to roll back.
	if count == 0 {
		err = setDBVersion(txn, 0)
	} else if count > 1 {
		err = fmt.Errorf("Version table contains more than one row.")
	}

	return err
}

// Returns the SQL statements base on whether we are using MySQL and the
// current and target DB version.
// This function assumes that currentVersion != targetVersion.
func getMigrations(isMySQL bool, currentVersion int, targetVersion int) [][]string {
	inc := util.SignInt(targetVersion - currentVersion)
	idx := currentVersion
	if inc < 0 {
		idx = currentVersion - 1
	}
	delta := util.AbsInt(targetVersion - currentVersion)
	result := make([][]string, 0, delta)

	for i := 0; i < delta; i++ {
		var temp []string
		switch {
		// using mysqlp
		case (inc > 0) && isMySQL:
			temp = migrationSteps[idx].MySQLUp
		case (inc < 0) && isMySQL:
			temp = migrationSteps[idx].MySQLDown
		// using sqlite
		case (inc > 0):
			temp = migrationSteps[idx].SQLiteUp
		case (inc < 0):
			temp = migrationSteps[idx].SQLiteDown
		}
		result = append(result, temp)
		idx += inc
	}
	return result
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []struct {
	MySQLUp    []string
	MySQLDown  []string
	SQLiteUp   []string
	SQLiteDown []string
}{
	// version 1
	{
		MySQLUp: []string{
			`CREATE TABLE IF NOT EXISTS shortcuts (
				id      INT             NOT NULL AUTO_INCREMENT PRIMARY KEY,
				traces  MEDIUMTEXT      NOT NULL
			)`,

			`CREATE TABLE IF NOT EXISTS clusters (
				id         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
				ts         BIGINT       NOT NULL,
				hash       TEXT         NOT NULL,
				regression FLOAT        NOT NULL,
				cluster    MEDIUMTEXT   NOT NULL,
				status     TEXT         NOT NULL,
				message    TEXT         NOT NULL
			)`,

			`CREATE TABLE IF NOT EXISTS tries (
				issue       VARCHAR(255) NOT NULL PRIMARY KEY,
				lastUpdated BIGINT       NOT NULL,
				results     LONGTEXT   NOT NULL
			)`,
		},
		MySQLDown: []string{},
		SQLiteUp: []string{
			`CREATE TABLE clusters (
				id         INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
				ts         TIMESTAMP    NOT NULL,
				hash       TEXT         NOT NULL,
				regression FLOAT        NOT NULL,
				cluster    MEDIUMTEXT   NOT NULL,
				status     TEXT         NOT NULL,
				message    TEXT         NOT NULL
			)`,
			`CREATE TABLE shortcuts (
				id      INTEGER     NOT NULL PRIMARY KEY AUTOINCREMENT,
				traces  MEDIUMTEXT  NOT NULL
			)`,
			`CREATE TABLE tries (
				issue       VARCHAR(255) NOT NULL PRIMARY KEY,
				lastUpdated TIMESTAMP    NOT NULL,
				results     MEDIUMTEXT   NOT NULL
			)`,
		},
		SQLiteDown: []string{
			`DROP TABLE IF EXISTS clusters`,
			`DROP TABLE IF EXISTS shortcuts`,
			`DROP TABLE IF EXISTS tries`,
		},
	},
	// version 2
	{
		MySQLUp: []string{
			`CREATE TABLE IF NOT EXISTS activitylog (
				id         INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
				timestamp  BIGINT       NOT NULL,
				userid     TEXT         NOT NULL,
				action     TEXT         NOT NULL,
				url        TEXT
			)`,
		},
		MySQLDown: []string{},
		SQLiteUp: []string{
			`CREATE TABLE activitylog (
				id         INTEGER      NOT NULL PRIMARY KEY AUTOINCREMENT,
				timestamp  TIMESTAMP    NOT NULL,
				userid     TEXT         NOT NULL,
				action     TEXT         NOT NULL,
				url        TEXT
			)`,
		},
		SQLiteDown: []string{
			`DROP TABLE IF EXISTS activitylog`,
		},
	},

	// Use this is a template for more migration steps.
	// version x
	// {
	// 	MySQLUp: ,
	// 	MySQLDown: ,
	// 	SQLiteUp: ,
	// 	SQLiteDown: ,
	// },
}
