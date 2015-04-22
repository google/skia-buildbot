package db

import "go.skia.org/infra/go/database"

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

	// version 2
	{
		MySQLUp: []string{
			`CREATE TABLE ignorerule (
				id            INT        NOT NULL AUTO_INCREMENT PRIMARY KEY,
				userid        TEXT       NOT NULL,
				expires       BIGINT     NOT NULL,
				query         TEXT       NOT NULL,
				note          TEXT       NOT NULL,
				INDEX expires_idx(expires)
			)`,
		},
		MySQLDown: []string{
			`DROP TABLE ignorerule`,
		},
	},

	// Use this is a template for more migration steps.
	// version 3
	{
		MySQLUp: []string{
			`CREATE TABLE exp_change (
				id            INT           NOT NULL AUTO_INCREMENT PRIMARY KEY,
				userid        VARCHAR(255)  NOT NULL,
				ts            BIGINT        NOT NULL,
				INDEX userid_idx(userid),
				INDEX ts_idx(ts)
			)`,
			`CREATE TABLE exp_test_change (
				changeid      INT           NOT NULL,
				name          VARCHAR(255)  NOT NULL,
				digest        VARCHAR(255)  NOT NULL,
				label         VARCHAR(255)  NOT NULL,
				removed       BIGINT,
				PRIMARY KEY (changeid, name, digest),
				INDEX expired_idx(removed)
			)`,
		},
		MySQLDown: []string{
			`DROP TABLE exp_test_change`,
			`DROP TABLE exp_change`,
		},
	},

	// version 4
	{
		MySQLUp: []string{
			`CREATE TABLE test_digest (
				name          VARCHAR(255)  NOT NULL,
				digest        VARCHAR(255)  NOT NULL,
				first         BIGINT        NOT NULL,
				last          BIGINT        NOT NULL,
				exception     VARCHAR(1024) NOT NULL,
				PRIMARY KEY (name, digest),
				INDEX first_idx (first),
				INDEX last_idx (last),
				INDEX exception_idx (exception)
			)`,
		},
		MySQLDown: []string{
			`DROP TABLE test_digest`,
		},
	},

	// Use this is a template for more migration steps.
	// version x
	// {
	// 	MySQLUp: ,
	// 	MySQLDown: ,
	// },
}
