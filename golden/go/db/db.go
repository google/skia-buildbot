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
			`DROP TABLE IF EXISTS expectations`,
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
			`DROP TABLE IF EXISTS ignorerule`,
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
			`DROP TABLE IF EXISTS exp_test_change`,
			`DROP TABLE IF EXISTS exp_change`,
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
			`DROP TABLE IF EXISTS test_digest`,
		},
	},

	// Remove unused tables.
	// version 5
	{
		MySQLUp: []string{
			`DROP TABLE IF EXISTS expectations`,
			`DROP TABLE IF EXISTS test_digest`,
		},
		MySQLDown: []string{},
	},

	// Add the undo field
	// version 6
	{
		MySQLUp: []string{
			`ALTER TABLE exp_change ADD undo_changeid INT NOT NULL DEFAULT 0`,
		},
		MySQLDown: []string{
			`ALTER TABLE exp_change DROP undo_changeid`,
		},
	},

	// Add a table to store trybot results.
	// version 7
	{
		MySQLUp: []string{
			`CREATE TABLE IF NOT EXISTS tries (
				issue        VARCHAR(255) NOT NULL PRIMARY KEY,
				last_updated BIGINT       NOT NULL,
				results      LONGTEXT     NOT NULL,
				INDEX tries_lastupdated_idx (last_updated)
			)`,
		},
		MySQLDown: []string{
			`DROP TABLE IF EXISTS tries`,
		},
	},

	// Use this is a template for more migration steps.
	// version x
	// {
	// 	MySQLUp: ,
	// 	MySQLDown: ,
	// },
}
