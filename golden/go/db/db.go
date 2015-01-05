package db

import "skia.googlesource.com/buildbot.git/go/database"

const (
	// Default database parameters.
	PROD_DB_HOST = "173.194.104.24"
	PROD_DB_PORT = 3306
	PROD_DB_NAME = "skiacorrectness"
)

// MigrationSteps returns the migration (up and down) for the database.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
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
	},

	// Use this is a template for more migration steps.
	// version x
	// {
	// 	MySQLUp: ,
	// 	MySQLDown: ,
	// },
}
