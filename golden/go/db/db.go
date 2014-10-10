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
)

// The migration steps for this DB.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}

// Returns the DB connection string for running in production where a
// metadata server is available. If 'local' is true it will always return
// "" (empty string). When used with Init() this will cause it to use a
// local SQLite database. If it's not local and the meta data server is
// unreachable it will terminate.
func GetDatabase(mySQLConnStr string, sqlitePath string, local bool) *database.DatabaseConfig {
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

// Define the migration steps.
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
