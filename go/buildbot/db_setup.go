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

	TABLE_BUILDS          = "builds2"
	TABLE_BUILD_REVISIONS = "buildRevisions2"
	TABLE_BUILD_STEPS     = "buildSteps2"
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
	`DROP INDEX idx_buildRevisions_revision_hash on buildRevisions;`,
}

var v3_up = []string{
	`CREATE TABLE IF NOT EXISTS builds2 (
		id          INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
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
		CONSTRAINT UNIQUE INDEX idx_builderMasterNumber (builder,master,number)
        )`,
	`CREATE TABLE IF NOT EXISTS buildRevisions2 (
		id       INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		buildId  INT UNSIGNED NOT NULL,
                revision VARCHAR(40)  NOT NULL,
                CONSTRAINT UNIQUE INDEX idx_revisionBuild (buildId, revision),
		INDEX idx_buildId (buildId),
		INDEX idx_revision (revision),
                FOREIGN KEY (buildId) REFERENCES builds2(id) ON DELETE CASCADE ON UPDATE CASCADE
        )`,
	`CREATE TABLE IF NOT EXISTS buildSteps2 (
		id           INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
		buildId      INT UNSIGNED NOT NULL,
                name         VARCHAR(100) NOT NULL,
                results      INT,
                number       INT          NOT NULL,
                started      DOUBLE,
                finished     DOUBLE,
		INDEX idx_buildId (buildId),
                FOREIGN KEY (buildId) REFERENCES builds2(id) ON DELETE CASCADE ON UPDATE CASCADE
        )`,
	`INSERT INTO builds2 (builder,master,number,gotRevision,branch,results,buildslave,started,finished,properties) SELECT builder,master,number,gotRevision,branch,results,buildslave,started,finished,properties FROM builds;`,
	`INSERT INTO buildRevisions2 (buildId,revision) SELECT t2.id, t1.revision FROM buildRevisions t1 INNER JOIN builds2 t2 ON (t1.builder = t2.builder AND t1.master = t2.master AND t1.number = t2.number);`,
	`INSERT INTO buildSteps2 (buildId,name,results,number,started,finished) SELECT t2.id, t1.name, t1.results, t1.number, t1.started, t1.finished FROM buildSteps t1 INNER JOIN builds2 t2 ON (t1.builder = t2.builder AND t1.master = t2.master AND t1.buildNumber = t2.number);`,
}

var v3_down = []string{
	`DROP TABLE IF EXISTS buildSteps2`,
	`DROP TABLE IF EXISTS buildRevisions2`,
	`DROP TABLE IF EXISTS builds2`,
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
	// version 3. Reformat tables.
	{
		MySQLUp:   v3_up,
		MySQLDown: v3_down,
	},
}

// MigrationSteps returns the database migration steps.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}
