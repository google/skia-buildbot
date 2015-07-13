package db

/*
	Store/Retrieve Cluster Telemetry Frontend data in a database.
*/

import (
	"github.com/jmoiron/sqlx"
	"go.skia.org/infra/go/database"
)

const (
	// Default database parameters.
	PROD_DB_HOST = "173.194.82.129"
	PROD_DB_PORT = 3306
	PROD_DB_NAME = "ctfe"

	TABLE_CHROMIUM_PERF_TASKS             = "ChromiumPerfTasks"
	TABLE_RECREATE_PAGE_SETS_TASKS        = "RecreatePageSetsTasks"
	TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS = "RecreateWebpageArchivesTasks"
)

var (
	DB *sqlx.DB = nil
)

// DatabaseConfig is a struct containing database configuration information.
type DatabaseConfig struct {
	*database.DatabaseConfig
}

// DBConfigFromFlags creates a DatabaseConfig from command-line flags.
func DBConfigFromFlags() *DatabaseConfig {
	return &DatabaseConfig{
		database.ConfigFromPrefixedFlags(PROD_DB_HOST, PROD_DB_PORT, database.USER_RW, PROD_DB_NAME, migrationSteps, "ctfe_"),
	}
}

// Setup the database to be shared across the app.
func (c *DatabaseConfig) InitDB() error {
	vdb, err := c.NewVersionedDB()
	if err != nil {
		return err
	}
	DB = sqlx.NewDb(vdb.DB, database.DEFAULT_DRIVER)
	return nil
}

var v1_up = []string{
	`CREATE TABLE IF NOT EXISTS ChromiumPerfTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		benchmark              VARCHAR(100) NOT NULL,
		platform               VARCHAR(100) NOT NULL,
		page_sets              VARCHAR(100) NOT NULL,
		repeat_runs            INT          NOT NULL,
		benchmark_args         VARCHAR(255),
		browser_args_nopatch   VARCHAR(255),
		browser_args_withpatch VARCHAR(255),
		description            VARCHAR(255),
		chromium_patch         TEXT,
		blink_patch            TEXT,
		skia_patch             TEXT,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1),
		nopatch_raw_output     VARCHAR(255),
		withpatch_raw_output   VARCHAR(255),
		results                VARCHAR(255)
	)`,
}

var v1_down = []string{
	`DROP TABLE IF EXISTS ChromiumPerfTasks`,
}

var v2_up = []string{
	`CREATE TABLE IF NOT EXISTS RecreatePageSetsTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		page_sets              VARCHAR(100) NOT NULL,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1)
	)`,
	`CREATE TABLE IF NOT EXISTS RecreateWebpageArchivesTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		page_sets              VARCHAR(100) NOT NULL,
		chromium_build         VARCHAR(100) NOT NULL,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1)
	)`,
}

var v2_down = []string{
	`DROP TABLE IF EXISTS RecreatePageSetsTasks`,
	`DROP TABLE IF EXISTS RecreateWebpageArchivesTasks`,
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
	// version 2. Create admin task tables.
	{
		MySQLUp:   v2_up,
		MySQLDown: v2_down,
	},
}

// MigrationSteps returns the database migration steps.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}
