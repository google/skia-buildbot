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
	TABLE_CAPTURE_SKPS_TASKS              = "CaptureSkpsTasks"
	TABLE_LUA_SCRIPT_TASKS                = "LuaScriptTasks"
	TABLE_CHROMIUM_BUILD_TASKS            = "ChromiumBuildTasks"
	TABLE_RECREATE_PAGE_SETS_TASKS        = "RecreatePageSetsTasks"
	TABLE_RECREATE_WEBPAGE_ARCHIVES_TASKS = "RecreateWebpageArchivesTasks"

	// From https://dev.mysql.com/doc/refman/5.0/en/storage-requirements.html
	TEXT_MAX_LENGTH      = 1<<16 - 1
	LONG_TEXT_MAX_LENGTH = int64(1<<32) - 1
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

var v3_up = []string{
	`CREATE TABLE IF NOT EXISTS ChromiumBuildTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		chromium_rev           VARCHAR(100) NOT NULL,
		chromium_rev_ts        BIGINT       NOT NULL,
		skia_rev               VARCHAR(100) NOT NULL,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1)
	)`,
}

var v3_down = []string{
	`DROP TABLE IF EXISTS ChromiumBuildTasks`,
}

var v4_up = []string{
	// Note: chromium_rev and skia_rev select a build from ChromiumBuildTasks; however, there is
	// no foreign-key constraint to allow flexibility in purging old Chromium builds indendently
	// of admin tasks.
	`ALTER TABLE RecreateWebpageArchivesTasks ADD (
		chromium_rev           VARCHAR(100),
		skia_rev               VARCHAR(100)
	)`,
	`UPDATE RecreateWebpageArchivesTasks SET
		chromium_rev = SUBSTRING_INDEX(chromium_build, '-', 1),
                skia_rev = SUBSTRING_INDEX(chromium_build, '-', -1)`,
	`ALTER TABLE RecreateWebpageArchivesTasks
		MODIFY chromium_rev	VARCHAR(100) NOT NULL,
		MODIFY skia_rev		VARCHAR(100) NOT NULL,
		DROP chromium_build`,
}

var v4_down = []string{
	`ALTER TABLE RecreateWebpageArchivesTasks ADD (
		chromium_build	VARCHAR(100)
	)`,
	`UPDATE RecreateWebpageArchivesTasks SET
		chromium_build = CONCAT(chromium_rev, '-', skia_rev)`,
	`ALTER TABLE RecreateWebpageArchivesTasks
		MODIFY chromium_build	VARCHAR(100) NOT NULL,
		DROP chromium_rev,
		DROP skia_rev`,
}

var v5_up = []string{
	// Note: similar to above, there is no foreign-key constraint on chromium_rev and skia_rev
	// to allow flexibility in purging old Chromium builds indendently of SKP repositories.
	`CREATE TABLE IF NOT EXISTS CaptureSkpsTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		page_sets              VARCHAR(100) NOT NULL,
		chromium_rev           VARCHAR(100) NOT NULL,
		skia_rev               VARCHAR(100) NOT NULL,
		description            VARCHAR(255) NOT NULL,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1)
	)`,
}

var v5_down = []string{
	`DROP TABLE IF EXISTS CaptureSkpsTasks`,
}

var v6_up = []string{
	// Note: similar to above, page_sets, chromium_rev, skia_rev select a SKP repository from
	// CaptureSkpsTasks; however, there is no foreign-key constraint to allow flexibility in
	// purging rows from CaptureSkpsTasks indendently of LuaScriptTasks.
	`CREATE TABLE IF NOT EXISTS LuaScriptTasks (
		id                     INT          NOT NULL AUTO_INCREMENT PRIMARY KEY,
		username               VARCHAR(255) NOT NULL,
		page_sets              VARCHAR(100) NOT NULL,
		chromium_rev           VARCHAR(100) NOT NULL,
		skia_rev               VARCHAR(100) NOT NULL,
		lua_script             TEXT NOT NULL,
		lua_aggregator_script  TEXT,
		description            VARCHAR(255) NOT NULL,
		ts_added               BIGINT       NOT NULL,
		ts_started             BIGINT,
		ts_completed           BIGINT,
		failure                TINYINT(1),
		script_output          VARCHAR(255),
                aggregated_output      VARCHAR(255)
	)`,
}

var v6_down = []string{
	`DROP TABLE IF EXISTS LuaScriptTasks`,
}

var v7_up = []string{
	`ALTER TABLE CaptureSkpsTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE ChromiumPerfTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE ChromiumBuildTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE LuaScriptTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE RecreatePageSetsTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
	`ALTER TABLE RecreateWebpageArchivesTasks ADD repeat_after_days BIGINT NOT NULL DEFAULT 0`,
}

var v7_down = []string{
	`ALTER TABLE CaptureSkpsTasks DROP repeat_after_days`,
	`ALTER TABLE ChromiumPerfTasks DROP repeat_after_days`,
	`ALTER TABLE ChromiumBuildTasks DROP repeat_after_days`,
	`ALTER TABLE LuaScriptTasks DROP repeat_after_days`,
	`ALTER TABLE RecreatePageSetsTasks DROP repeat_after_days`,
	`ALTER TABLE RecreateWebpageArchivesTasks DROP repeat_after_days`,
}

var v8_up = []string{
	`ALTER TABLE ChromiumPerfTasks ADD run_in_parallel BOOLEAN NOT NULL DEFAULT False`,
}

var v8_down = []string{
	`ALTER TABLE ChromiumPerfTasks DROP run_in_parallel`,
}

var v9_up = []string{
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN chromium_patch longtext`,
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN blink_patch longtext`,
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN skia_patch longtext`,
}

var v9_down = []string{
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN chromium_patch text`,
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN blink_patch text`,
	`ALTER TABLE ChromiumPerfTasks MODIFY COLUMN skia_patch text`,
}

var v10_up = []string{
	`ALTER TABLE ChromiumPerfTasks CONVERT TO CHARACTER SET utf32`,
}

var v10_down = []string{
	`ALTER TABLE ChromiumPerfTasks CONVERT TO CHARACTER SET utf32`,
}

// Define the migration steps.
// Note: Only add to this list, once a step has landed in version control it
// must not be changed.
var migrationSteps = []database.MigrationStep{
	// version 1. Create Chromium Perf tables.
	{
		MySQLUp:   v1_up,
		MySQLDown: v1_down,
	},
	// version 2. Create Admin Task tables.
	{
		MySQLUp:   v2_up,
		MySQLDown: v2_down,
	},
	// version 3. Create Chromium Build table.
	{
		MySQLUp:   v3_up,
		MySQLDown: v3_down,
	},
	// version 4. Modify Chromium Build columns.
	{
		MySQLUp:   v4_up,
		MySQLDown: v4_down,
	},
	// version 5. Create Capture SKPs table.
	{
		MySQLUp:   v5_up,
		MySQLDown: v5_down,
	},
	// version 6. Create Lua Scripts table.
	{
		MySQLUp:   v6_up,
		MySQLDown: v6_down,
	},
	// version 7: Add repeat_after_days column to all tables.
	{
		MySQLUp:   v7_up,
		MySQLDown: v7_down,
	},
	// version 8: Add run_in_parallel column to ChromiumPerfTasks table.
	{
		MySQLUp:   v8_up,
		MySQLDown: v8_down,
	},
	// version 9: Change chromium_patch, blink_patch and skia_patch to longtext in ChromiumPerfTasks table.
	{
		MySQLUp:   v9_up,
		MySQLDown: v9_down,
	},
	// version 10: Convert character set in ChromiumPerfTasks from utf8 to utf32.
	{
		MySQLUp:   v10_up,
		MySQLDown: v10_down,
	},
}

// MigrationSteps returns the database migration steps.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
}
