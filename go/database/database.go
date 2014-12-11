package database

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/glog"
	_ "github.com/mattn/go-sqlite3"
	"skia.googlesource.com/buildbot.git/go/util"
)

// Config information to create a database connection.
type DatabaseConfig struct {
	MySQLString    string
	SQLiteFilePath string
	MigrationSteps []MigrationStep
}

// Single step to migrated from one database version to the next and back.
type MigrationStep struct {
	MySQLUp    []string
	MySQLDown  []string
	SQLiteUp   []string
	SQLiteDown []string
}

// Database handle to send queries to the underlying database.
type VersionedDB struct {
	// Database intance that is either backed by SQLite or MySQl.
	DB *sql.DB

	// Keeps track if we are connected to MySQL or SQLite
	IsMySQL bool

	// List of migration steps for this database.
	migrationSteps []MigrationStep
}

// Init must be called once before DB is used.
//
// Since it used glog, make sure it is also called after flag.Parse is called.
func NewVersionedDB(conf *DatabaseConfig) *VersionedDB {
	// If there is a connection string then connect to the MySQL server.
	// This is for testing only. In production we get the relevant information
	// from the metadata server.
	var err error
	var isMySQL = true
	var DB *sql.DB = nil

	if conf.MySQLString != "" {
		glog.Infoln("Opening SQL database.")
		if DB, err = sql.Open("mysql", conf.MySQLString); err == nil {
			glog.Infoln("Sending Ping.")
			err = DB.Ping()
		}

		if err != nil {
			glog.Fatalln("Failed to open connection to SQL server:", err)
		}
	} else {
		// Open a local SQLite database instead.
		glog.Infof("Opening local sqlite database at: %s", conf.SQLiteFilePath)
		// Fallback to sqlite for local use.
		DB, err = sql.Open("sqlite3", conf.SQLiteFilePath)
		if err != nil {
			glog.Fatalln("Failed to open:", err)
		}
		isMySQL = false
	}

	result := &VersionedDB{
		DB:             DB,
		IsMySQL:        isMySQL,
		migrationSteps: conf.MigrationSteps,
	}

	// Make sure the migration table exists.
	if err := result.checkVersionTable(); err != nil {
		// We are using panic() instead of Fataln() to be able to trap this
		// in tests and make sure it fails when no version table exists.
		glog.Errorln("Unable to create version table.")
		panic("Attempt to create version table returned: " + err.Error())
	}
	glog.Infoln("Version table OK.")

	// Migrate to the latest version if we are using SQLite, so we don't have
	// to run the *_migratdb command for a local database.
	if !result.IsMySQL {
		result.Migrate(result.MaxDBVersion())
	}

	// Ping the database occasionally to keep the connection fresh.
	go func() {
		c := time.Tick(1 * time.Minute)
		for _ = range c {
			if err := result.DB.Ping(); err != nil {
				glog.Warningln("Database failed to respond:", err)
			}
			glog.Infof("db: Successful ping")
		}
	}()

	return result
}

// Close the underlying database.
func (vdb *VersionedDB) Close() error {
	return vdb.DB.Close()
}

// Migrates the database to the specified target version. Use DBVersion() to
// retrieve the current version of the database.
func (vdb *VersionedDB) Migrate(targetVersion int) error {
	if (targetVersion < 0) || (targetVersion > vdb.MaxDBVersion()) {
		glog.Fatalf("Target db version must be in range: [0 .. %d]", vdb.MaxDBVersion())
	}

	currentVersion, err := vdb.DBVersion()
	if err != nil {
		return err
	}

	if targetVersion == currentVersion {
		return nil
	}

	// start a transaction
	txn, err := vdb.DB.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			glog.Errorf("Rolling back commit. Error: %s", err)
			txn.Rollback()
		} else {
			glog.Infoln("Committing changes.")
			txn.Commit()
		}
	}()

	// run through the transactions
	runSteps := vdb.getMigrations(currentVersion, targetVersion)
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
	if err = vdb.setDBVersion(txn, targetVersion); err != nil {
		return err
	}

	return nil
}

// Returns the current version of the database. It assumes that the
// Migrate function has already been called and the version table has been
// created in the database.
func (vdb *VersionedDB) DBVersion() (int, error) {
	stmt := `
		SELECT version
		FROM sk_db_version
		WHERE id=1`

	var version int
	err := vdb.DB.QueryRow(stmt).Scan(&version)
	return version, err
}

// Returns the highest version currently available.
func (vdb *VersionedDB) MaxDBVersion() int {
	return len(vdb.migrationSteps)
}

// Returns an error if the version table does not exist.
func (vdb *VersionedDB) checkVersionTable() error {
	// Check if the table exists in MySQL or SQLite.
	stmt := "SHOW TABLES LIKE 'sk_db_version'"
	if !vdb.IsMySQL {
		stmt = "SELECT name FROM sqlite_master WHERE type='table' AND name='sk_db_version';"
	}

	var temp string
	err := vdb.DB.QueryRow(stmt).Scan(&temp)
	if err != nil {
		// See if we can create the version table.
		return vdb.ensureVersionTable()
	}

	return nil
}

func (vdb *VersionedDB) setDBVersion(txn *sql.Tx, newDBVersion int) error {
	stmt := `REPLACE INTO sk_db_version (id, version, updated) VALUES(1, ?, ?)`
	_, err := txn.Exec(stmt, newDBVersion, time.Now().Unix())
	return err
}

func (vdb *VersionedDB) ensureVersionTable() error {
	txn, err := vdb.DB.Begin()
	defer func() {
		if err != nil {
			glog.Errorf("Encountered error rolling back: %s", err)
			txn.Rollback()
		} else {
			txn.Commit()
		}
	}()

	if err != nil {
		fmt.Errorf("Unable to start database transaction. %s", err)
	}

	stmt := `CREATE TABLE IF NOT EXISTS sk_db_version (
			id         INTEGER      NOT NULL PRIMARY KEY,
			version    INTEGER      NOT NULL,
			updated    BIGINT       NOT NULL
		)`
	if _, err = txn.Exec(stmt); err != nil {
		return fmt.Errorf("Creating version table failed: %s", err)
	}

	stmt = "SELECT COUNT(*) FROM sk_db_version"
	var count int
	if err = txn.QueryRow(stmt).Scan(&count); err != nil {
		return fmt.Errorf("Unable to read version table: %s", err)
	}

	// In both cases we want the transaction to roll back.
	if count == 0 {
		err = vdb.setDBVersion(txn, 0)
	} else if count > 1 {
		err = fmt.Errorf("Version table contains more than one row.")
	}

	return err
}

// Returns the SQL statements base on whether we are using MySQL and the
// current and target DB version.
// This function assumes that currentVersion != targetVersion.
func (vdb *VersionedDB) getMigrations(currentVersion int, targetVersion int) [][]string {
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
		case (inc > 0) && vdb.IsMySQL:
			temp = vdb.migrationSteps[idx].MySQLUp
		case (inc < 0) && vdb.IsMySQL:
			temp = vdb.migrationSteps[idx].MySQLDown
		// using sqlite
		case (inc > 0):
			temp = vdb.migrationSteps[idx].SQLiteUp
		case (inc < 0):
			temp = vdb.migrationSteps[idx].SQLiteDown
		}
		result = append(result, temp)
		idx += inc
	}
	return result
}
