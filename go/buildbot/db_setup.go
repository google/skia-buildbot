package buildbot

/*
	Store/Retrieve buildbot data in a database.
*/

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	"skia.googlesource.com/buildbot.git/go/database"
)

const (
	// Default database parameters.
	PROD_DB_HOST = "173.194.253.125"
	PROD_DB_PORT = 3306
	PROD_DB_NAME = "buildbot"

	TABLE_BUILDS          = "builds"
	TABLE_BUILD_REVISIONS = "buildRevisions"
	TABLE_BUILD_STEPS     = "buildSteps"
)

var (
	DB *sqlx.DB = nil
)

// Setup the database to be shared across the app.
func InitDB(conf *database.DatabaseConfig) error {
	db, err := sqlx.Open("mysql", conf.MySQLString)
	if err != nil {
		return fmt.Errorf("Failed to open database: %v", err)
	}
	DB = db
	return nil
}

var v1_up = []string{
	`CREATE TABLE IF NOT EXISTS builds (
		builder     VARCHAR(100) NOT NULL,
		master      VARCHAR(100) NOT NULL,
		number      INT          NOT NULL,
		gotRevision VARCHAR(40),
		branch      VARCHAR(100) NOT NULL,
		results     INT,
		buildslave  VARCHAR(100) NOT NULL,
		started     DOUBLE,
		finished    DOUBLE,
		properties  TEXT,
		CONSTRAINT pk_builderMasterNumber PRIMARY KEY (builder,master,number)
	)`,
	`CREATE TABLE IF NOT EXISTS buildRevisions (
		revision VARCHAR(40)  NOT NULL,
		builder  VARCHAR(100) NOT NULL,
		master   VARCHAR(100) NOT NULL,
		number   INT          NOT NULL,
		CONSTRAINT pk_revisionBuild PRIMARY KEY (revision,builder,master,number),
		FOREIGN KEY (builder,master,number) REFERENCES builds(builder,master,number) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS buildSteps (
		builder      VARCHAR(100) NOT NULL,
		master       VARCHAR(100) NOT NULL,
		buildNumber  INT          NOT NULL,
		name         VARCHAR(100) NOT NULL,
		results      INT,
		number       INT          NOT NULL,
		started      DOUBLE,
		finished     DOUBLE,
		FOREIGN KEY (builder,master,buildNumber) REFERENCES builds(builder,master,number) ON DELETE CASCADE ON UPDATE CASCADE
	)`,
}

var v1_down = []string{
	`DROP TABLE IF EXISTS buildSteps`,
	`DROP TABLE IF EXISTS buildRevisions`,
	`DROP TABLE IF EXISTS builds`,
}

var v2_up = []string{
	`CREATE INDEX idx_buildRevisions_builderMasterNumber_hash ON buildRevisions(builder,master,number) USING HASH;`,
	`CREATE INDEX idx_buildRevisions_revision_hash ON buildRevisions(revision) USING HASH;`,
	`CREATE INDEX idx_buildSteps_builderMasterNumber_hash ON buildSteps(builder,master,buildNumber) USING HASH;`,
	`CREATE INDEX idx_builds_builderMasterNumber_hash ON builds(builder,master,number) USING HASH;`,
}

var v2_down = []string{
	`DROP INDEX idx_buildRevisions_builderMasterNumber_hash ON buildRevisions;`,
	`DROP INDEX idx_buildRevisions_revision_hash on buildRevisions;`,
	`DROP INDEX idx_buildSteps_builderMasterNumber_hash on buildSteps;`,
	`DROP INDEX idx_builds_builderMasterNumber_hash on builds;`,
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
	// version 1. Create tables.
	{
		MySQLUp:   v1_up,
		MySQLDown: v1_down,
	},
	// version 2. Create indices.
	{
		MySQLUp:   v2_up,
		MySQLDown: v2_down,
	},
}

// MigrationSteps returns the database migration steps.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}
