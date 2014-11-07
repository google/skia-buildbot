package db

import (
	"fmt"

	"github.com/golang/glog"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/metadata"
)

const (
	// Key of the password for the readwrite user.
	METADATA_KEY = "readwrite"

	// Detfault database parameters.
	DEFAULT_DB_HOST = "173.194.104.24:3306"
	DEFAULT_DB_PORT = "3306"
	DEFAULT_DB_NAME = "skiacorrectness"

	// Template to generate the MySQL database connection string.
	// And 3306 is the default port for MySQL.
	DB_CONN_TMPL = "%s:%s@tcp(%s:%s)/%s?parseTime=true"
)

// MigrationSteps returns the migration (up and down) for the database.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}

// GetConfig returns a DatabaseConfig instance for running in production if a
// metadata server is available. If 'local' is true it will always
// set the MySQL connection string to "" and thus use a local SQLite database
// when used with database.NewVersionedDB.
func GetConfig(mySQLConnStr string, sqlitePath string, local bool) *database.DatabaseConfig {
	useMySQLConnStr := mySQLConnStr

	// We are in the production environment, so we look up the password.
	if !local {
		//  First, get the password from the metadata server.
		// See https://developers.google.com/compute/docs/metadata#custom.
		password, err := metadata.Get(METADATA_KEY)
		if err != nil {
			glog.Fatalf("Failed to find metadata. Use 'local' flag when running locally.")
		}
		useMySQLConnStr = fmt.Sprintf(mySQLConnStr, password)
	}

	return &database.DatabaseConfig{
		MySQLString:    useMySQLConnStr,
		SQLiteFilePath: sqlitePath,
		MigrationSteps: migrationSteps,
	}
}

// GetConnectionString returns a MySQL connection string with the given
// parameters replace in the template. Only userName has to be provided.
// If host, port or dbName are empty the default (production) values will
// be used.
func GetConnectionString(userName, host, port, dbName string) string {
	useHost, usePort, useDBName := host, port, dbName
	if useHost == "" {
		useHost = DEFAULT_DB_HOST
	}

	if usePort == "" {
		usePort = DEFAULT_DB_PORT
	}

	if useDBName == "" {
		useDBName = DEFAULT_DB_NAME
	}

	return fmt.Sprintf(DB_CONN_TMPL, userName, "%s", useHost, usePort, useDBName)
}

// migrationSteps define the steps it takes to migrate the db between versions.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
	// version 1
	{
		MySQLUp: []string{
			`CREATE TABLE expectations (
				id            INT        NOT NULL AUTO_INCREMENT PRIMARY KEY,
				userid        TEXT       NOT NULL,
				ts            BIGINT     NOT NULL,
				expectations  MEDIUMTEXT NOT NULL
			)`,
		},
		MySQLDown: []string{
			`DROP TABLE expectations`,
		},
		SQLiteUp: []string{
			`CREATE TABLE expectations (
				id            INTEGER     NOT NULL PRIMARY KEY AUTOINCREMENT,
				userid        TEXT        NOT NULL,
				ts            BIGINT      NOT NULL,
				expectations  MEDIUXMTEXT  NOT NULL
			)`,
		},
		SQLiteDown: []string{
			`DROP TABLE expectations`,
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
