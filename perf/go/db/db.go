package db

import (
	"database/sql"

	"fmt"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/metadata"
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
	DB_CONN_TMPL = "%s:%s@tcp(173.194.104.24:3306)/skia?parseTime=true"

	// Username of the read/write user.
	RW_USER = "readwrite"
)

var (
	DB *sql.DB = nil
)

// Setup the database to be shared across the app.
func Init(conf *database.DatabaseConfig) {
	vdb := database.NewVersionedDB(conf)
	DB = vdb.DB
}

func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}

// Returns the DB connection string for running in production where a
// metadata server is available. If 'local' is true it will always return
// "" (empty string). When used with Init() this will cause it to use a
// local SQLite database. If it's not local and the meta data server is
// unreachable it will terminate.
func ProdDatabaseConfig(local bool) *database.DatabaseConfig {
	mysqlStr := ""
	sqlitePath := SQLITE_DB_PATH

	// We are in the production environment, so we look up the parameters.
	if !local {
		//  First, get the password from the metadata server.
		// See https://developers.google.com/compute/docs/metadata#custom.
		password, err := metadata.Get(METADATA_KEY)
		if err != nil {
			glog.Fatalf("Failed to find metadata. Use 'local' flag when running locally.")
		}
		mysqlStr, sqlitePath = fmt.Sprintf(DB_CONN_TMPL, RW_USER, password), ""
	}

	return &database.DatabaseConfig{
		MySQLString:    mysqlStr,
		SQLiteFilePath: sqlitePath,
		MigrationSteps: migrationSteps,
	}
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
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
