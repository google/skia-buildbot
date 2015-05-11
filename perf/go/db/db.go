package db

import (
	"database/sql"

	"go.skia.org/infra/go/database"
)

const (
	// Default database parameters.
	PROD_DB_HOST = "173.194.104.24"
	PROD_DB_PORT = 3306
	PROD_DB_NAME = "skia"
)

var (
	DB *sql.DB = nil
)

// DatabaseConfig is a struct containing database configuration information.
type DatabaseConfig struct {
	*database.DatabaseConfig
}

// DBConfigFromFlags creates a DatabaseConfig from command-line flags.
func DBConfigFromFlags() *DatabaseConfig {
	return &DatabaseConfig{
		database.ConfigFromFlags(PROD_DB_HOST, PROD_DB_PORT, database.USER_RW, PROD_DB_NAME, migrationSteps),
	}
}

// Setup the database to be shared across the app.
func (c *DatabaseConfig) InitDB() error {
	vdb, err := c.NewVersionedDB()
	if err != nil {
		return err
	}
	DB = vdb.DB
	return nil
}

// MigrationSteps returns the migration (up and down) for the database.
func MigrationSteps() []database.MigrationStep {
	return migrationSteps
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
	},

	// Use this is a template for more migration steps.
	// version x
	// {
	// 	MySQLUp: ,
	// 	MySQLDown: ,
	// },
}
